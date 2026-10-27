package internal

import (
	"context"
	"sort"
	"strings"

	"github.com/bporter816/aws-tui/internal/ui"
	"github.com/bporter816/aws-tui/internal/utils"
)

type TableView[T any] struct {
	*ui.Table
	service    string
	labels     []string
	items      []T // in display order
	loaded     []T // in load order
	rows       [][]string
	order      []int // display index -> index into loaded
	sortCol    int
	sortDesc   bool
	filter     string
	load       func(context.Context) ([]T, error)
	toRow      func(T) []string
	keyActions []KeyAction
}

func NewTableView[T any](
	service string,
	labels []string,
	columns []string,
	load func(context.Context) ([]T, error),
	toRow func(T) []string,
	keyActions func(*TableView[T]) []KeyAction,
) *TableView[T] {
	v := &TableView[T]{
		Table:   ui.NewTable(columns, 1, 0),
		service: service,
		labels:  labels,
		sortCol: -1,
		load:    load,
		toRow:   toRow,
	}
	if keyActions != nil {
		v.keyActions = keyActions(v)
	}
	return v
}

func (t *TableView[T]) GetService() string         { return t.service }
func (t *TableView[T]) GetLabels() []string        { return t.labels }
func (t *TableView[T]) GetKeyActions() []KeyAction { return t.keyActions }

func (t *TableView[T]) Selected() (T, error) {
	row, err := t.GetRowSelection()
	if err != nil {
		var zero T
		return zero, err
	}
	return t.items[row-1], nil
}

func (t *TableView[T]) Render(ctx context.Context) error {
	items, err := t.load(ctx)
	if err != nil {
		return err
	}
	t.loaded = items
	t.rows = make([][]string, len(items))
	for i, item := range items {
		t.rows[i] = t.toRow(item)
	}
	t.apply(-1)
	return nil
}

func (t *TableView[T]) GetSortColumns() []string { return t.GetHeaders() }
func (t *TableView[T]) GetSort() (int, bool)     { return t.sortCol, t.sortDesc }

// SetSort sorts the table by column col (descending if desc is set), keeping
// the currently selected item selected. A negative col restores load order.
func (t *TableView[T]) SetSort(col int, desc bool) {
	selected := t.selectedIndex()
	t.sortCol, t.sortDesc = col, desc
	t.apply(selected)
}

func (t *TableView[T]) GetFilter() string { return t.filter }

// SetFilter shows only the rows that contain query (case-insensitive) in any
// column. An empty query shows all rows.
func (t *TableView[T]) SetFilter(query string) {
	selected := t.selectedIndex()
	t.filter = query
	if !t.apply(selected) {
		t.Select(1, 0)
	}
}

// selectedIndex returns the index into the loaded items of the selected row,
// or -1 if nothing is selected.
func (t *TableView[T]) selectedIndex() int {
	if row, err := t.GetRowSelection(); err == nil && row-1 < len(t.order) {
		return t.order[row-1]
	}
	return -1
}

func (t *TableView[T]) matchesFilter(row []string, query string) bool {
	for _, cell := range row {
		if strings.Contains(strings.ToLower(cell), query) {
			return true
		}
	}
	return false
}

// apply rebuilds the displayed rows from the loaded ones using the current
// filter and sort order. If selected is the index of a loaded item that is
// still displayed, its row gets selected and apply returns true.
func (t *TableView[T]) apply(selected int) bool {
	query := strings.ToLower(t.filter)
	t.order = make([]int, 0, len(t.loaded))
	for i, row := range t.rows {
		if query == "" || t.matchesFilter(row, query) {
			t.order = append(t.order, i)
		}
	}
	if t.sortCol >= 0 {
		cell := func(i int) string {
			if row := t.rows[i]; t.sortCol < len(row) {
				return row[t.sortCol]
			}
			return ""
		}
		sort.SliceStable(t.order, func(i, j int) bool {
			a, b := cell(t.order[i]), cell(t.order[j])
			// keep empty values last regardless of direction
			if t.sortDesc && a != "" && b != "" {
				a, b = b, a
			}
			return utils.CompareValues(a, b) < 0
		})
	}

	t.items = make([]T, len(t.order))
	rows := make([][]string, len(t.order))
	found := false
	for i, idx := range t.order {
		t.items[i] = t.loaded[idx]
		rows[i] = t.rows[idx]
		if idx == selected {
			t.Select(i+1, 0)
			found = true
		}
	}
	t.SetData(rows)
	t.SetSortIndicator(t.sortCol, t.sortDesc)
	return found
}
