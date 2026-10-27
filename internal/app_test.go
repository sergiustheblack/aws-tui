package internal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newTestApplication runs an Application with the given component on a
// simulated screen. It skips the header, which needs AWS access.
func newTestApplication(t *testing.T, c Component) (*Application, tcell.SimulationScreen) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(100, 30)

	a := &Application{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
	}
	a.ctx, a.cancel = context.WithCancel(context.Background())
	a.footer = NewFooter(a)
	a.layout = tview.NewFlex().SetDirection(tview.FlexRow)
	a.layout.AddItem(a.pages, 0, 1, true)
	a.layout.AddItem(a.footer, 1, 0, false)
	a.app.SetScreen(screen).SetRoot(a.layout, true).SetFocus(a.pages)
	a.app.SetInputCapture(a.handleKey)

	addComponent(t, a, c)

	done := make(chan error, 1)
	go func() { done <- a.app.Run() }()
	t.Cleanup(func() {
		a.app.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("application did not stop")
		}
	})
	return a, screen
}

func addComponent(t *testing.T, a *Application, c Component) {
	t.Helper()
	if err := c.Render(a.ctx); err != nil {
		t.Fatal(err)
	}
	a.components = append(a.components, c)
	a.pages.AddAndSwitchToPage(fmt.Sprintf("%v", len(a.components)), c, true)
}

// sync waits until the application has processed all queued events, failing
// the test if it hangs.
func sync(t *testing.T, a *Application) {
	t.Helper()
	ch := make(chan struct{})
	go a.app.QueueUpdate(func() { close(ch) })
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("application is not responding")
	}
}

func typeKeys(t *testing.T, a *Application, screen tcell.SimulationScreen, keys ...any) {
	t.Helper()
	for _, k := range keys {
		switch k := k.(type) {
		case string:
			for _, r := range k {
				screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
				sync(t, a)
			}
		case tcell.Key:
			screen.InjectKey(k, 0, tcell.ModNone)
			sync(t, a)
		}
	}
}

func TestApplicationFilterInput(t *testing.T) {
	v := newTestTableView([][]string{
		{"web-1", "10.0.0.1"},
		{"db-1", "10.0.1.5"},
	})
	a, screen := newTestApplication(t, v)

	typeKeys(t, a, screen, "/", "db")
	if got := v.GetFilter(); got != "db" {
		t.Fatalf("expected filter db, got %q", got)
	}
	typeKeys(t, a, screen, tcell.KeyEnter)
	if a.filterInput != nil {
		t.Errorf("expected filter input to be closed")
	}
	if got := displayedCol(v, 0); len(got) != 1 || got[0] != "db-1" {
		t.Errorf("expected only db-1, got %v", got)
	}

	// Escape in the table clears the filter
	typeKeys(t, a, screen, tcell.KeyEscape)
	if got := v.GetFilter(); got != "" {
		t.Errorf("expected filter to be cleared, got %q", got)
	}

	// Escape in the input clears the filter too
	typeKeys(t, a, screen, "/", "web", tcell.KeyEscape)
	if got := v.GetFilter(); got != "" || a.filterInput != nil {
		t.Errorf("expected filter to be cleared and input closed, got %q", got)
	}
}

// Switching region or profile replaces all pages, which must not leave the
// pages claiming focus over the filter input.
func TestApplicationFilterInputAfterRebuild(t *testing.T) {
	a, screen := newTestApplication(t, newTestTableView([][]string{{"web-1", ""}}))

	v := newTestTableView([][]string{
		{"web-1", "10.0.0.1"},
		{"db-1", "10.0.1.5"},
	})
	a.app.QueueUpdate(func() {
		a.clearPages()
		addComponent(t, a, v)
	})
	sync(t, a)

	typeKeys(t, a, screen, "/", "db")
	if got := v.GetFilter(); got != "db" {
		t.Fatalf("expected filter db, got %q", got)
	}
	typeKeys(t, a, screen, tcell.KeyEscape)
	if a.filterInput != nil {
		t.Errorf("expected filter input to be closed")
	}
}
