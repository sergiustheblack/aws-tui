package internal

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

type Footer struct {
	*tview.TextView
	app *Application
}

func NewFooter(app *Application) *Footer {
	f := &Footer{
		TextView: tview.NewTextView().SetDynamicColors(true),
		app:      app,
	}
	return f
}

func (f Footer) Render() {
	var names []string
	for i, v := range f.app.components {
		if i < 2 {
			names = append(names, v.GetService())
		}
		names = append(names, v.GetLabels()...)
	}
	str := strings.Join(names, " > ")
	if n := len(f.app.components); n > 0 {
		if v, ok := f.app.components[n-1].(Filterable); ok && v.GetFilter() != "" {
			str += fmt.Sprintf("  [yellow]/%v[white]", tview.Escape(v.GetFilter()))
		}
	}
	f.SetText(str)
}
