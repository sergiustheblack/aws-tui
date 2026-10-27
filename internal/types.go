package internal

import (
	"context"

	"github.com/rivo/tview"
)

type Component interface {
	tview.Primitive
	GetService() string
	GetLabels() []string
	GetKeyActions() []KeyAction
	Render(context.Context) error
}

// Sortable is implemented by components whose rows can be sorted by column.
type Sortable interface {
	GetSortColumns() []string
	GetSort() (col int, desc bool)
	SetSort(col int, desc bool)
}

// Filterable is implemented by components whose rows can be filtered by a
// search query.
type Filterable interface {
	GetFilter() string
	SetFilter(query string)
}
