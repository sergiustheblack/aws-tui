package internal

import (
	"context"
	"reflect"
	"testing"
)

func newTestTableView(items [][]string) *TableView[[]string] {
	return NewTableView(
		"Test",
		[]string{"Items"},
		[]string{"NAME", "IP"},
		func(context.Context) ([][]string, error) { return items, nil },
		func(v []string) []string { return v },
		nil,
	)
}

func displayedCol(v *TableView[[]string], col int) []string {
	var out []string
	for r := 1; r < v.GetRowCount(); r++ {
		out = append(out, v.GetCell(r, col).Text)
	}
	return out
}

func TestTableViewSort(t *testing.T) {
	v := newTestTableView([][]string{
		{"b", "10.0.0.10"},
		{"c", ""},
		{"a", "10.0.0.9"},
	})
	if err := v.Render(context.Background()); err != nil {
		t.Fatal(err)
	}

	v.SetSort(1, false)
	if got, want := displayedCol(v, 1), []string{"10.0.0.9", "10.0.0.10", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("ascending: expected %v, got %v", want, got)
	}

	v.SetSort(1, true)
	if got, want := displayedCol(v, 1), []string{"10.0.0.10", "10.0.0.9", ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("descending: expected %v, got %v", want, got)
	}
	if got := v.GetCell(0, 1).Text; got != "IP ↓" {
		t.Errorf("expected sort indicator in header, got %q", got)
	}

	// selection follows the item and Selected() matches the displayed row
	v.Select(1, 0)
	v.SetSort(0, false)
	if got, want := displayedCol(v, 0), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("by name: expected %v, got %v", want, got)
	}
	sel, err := v.Selected()
	if err != nil {
		t.Fatal(err)
	}
	if sel[0] != "b" {
		t.Errorf("expected selection to stay on b, got %v", sel[0])
	}
	if name, _ := v.GetColSelection("NAME"); name != "b" {
		t.Errorf("expected selected NAME cell b, got %v", name)
	}

	// sort is kept across refresh, and default order can be restored
	if err := v.Render(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, want := displayedCol(v, 0), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after refresh: expected %v, got %v", want, got)
	}
	v.SetSort(-1, false)
	if got, want := displayedCol(v, 0), []string{"b", "c", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("default order: expected %v, got %v", want, got)
	}
	if got := v.GetCell(0, 1).Text; got != "IP" {
		t.Errorf("expected no sort indicator, got %q", got)
	}
}

func TestTableViewFilter(t *testing.T) {
	v := newTestTableView([][]string{
		{"web-1", "10.0.0.1"},
		{"db-1", "10.0.1.5"},
		{"Web-2", "10.0.0.2"},
	})
	if err := v.Render(context.Background()); err != nil {
		t.Fatal(err)
	}
	v.Select(3, 0)

	v.SetFilter("WEB")
	if got, want := displayedCol(v, 0), []string{"web-1", "Web-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filter by name: expected %v, got %v", want, got)
	}
	if sel, _ := v.Selected(); sel[0] != "Web-2" {
		t.Errorf("expected selection to stay on Web-2, got %v", sel[0])
	}

	// any column matches, and sorting applies to the filtered rows
	v.SetFilter("10.0.0")
	v.SetSort(0, true)
	if got, want := displayedCol(v, 0), []string{"Web-2", "web-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filter by ip: expected %v, got %v", want, got)
	}

	v.SetFilter("nothing")
	if got := displayedCol(v, 0); len(got) != 0 {
		t.Errorf("expected no rows, got %v", got)
	}
	if _, err := v.Selected(); err == nil {
		t.Errorf("expected no selection with no rows")
	}

	v.SetFilter("")
	if got := displayedCol(v, 0); len(got) != 3 {
		t.Errorf("expected all rows after clearing filter, got %v", got)
	}
}
