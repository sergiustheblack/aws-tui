package internal

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/bporter816/aws-tui/internal/model"
	"github.com/bporter816/aws-tui/internal/repo"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// recordInput is what the user enters in the record form.
type recordInput struct {
	Name   string
	Type   string
	TTL    string
	Values string // one value per line
}

func recordInputFrom(r model.Route53Record) recordInput {
	in := recordInput{
		Name: strings.TrimSuffix(aws.ToString(r.Name), "."),
		Type: string(r.Type),
	}
	if r.TTL != nil {
		in.TTL = strconv.FormatInt(*r.TTL, 10)
	}
	var values []string
	for _, v := range r.ResourceRecords {
		values = append(values, aws.ToString(v.Value))
	}
	in.Values = strings.Join(values, "\n")
	return in
}

// qualifyRecordName turns name into a fully qualified name in zone. Names
// ending with a dot are taken as is, "@" or an empty name is the zone apex,
// and other names outside the zone get the zone name appended.
func qualifyRecordName(name, zone string) string {
	name = strings.TrimSpace(name)
	zone = strings.TrimSuffix(zone, ".")
	if strings.HasSuffix(name, ".") {
		return name
	}
	if name == "" || name == "@" {
		return zone + "."
	}
	lower, lowerZone := strings.ToLower(name), strings.ToLower(zone)
	if lower == lowerZone || strings.HasSuffix(lower, "."+lowerZone) {
		return name + "."
	}
	return name + "." + zone + "."
}

func parseRecordValues(s string) []r53Types.ResourceRecord {
	var records []r53Types.ResourceRecord
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			records = append(records, r53Types.ResourceRecord{Value: aws.String(line)})
		}
	}
	return records
}

// buildRecordChanges returns the changes that turn old into a record made from
// in, or create a new one if old is nil. An edit deletes the old record and
// creates the new one in the same batch, so it fails instead of overwriting
// the record if someone changed it in the meantime. Settings the form doesn't
// show (routing policy, health check) are kept. No changes means there is
// nothing to do.
func buildRecordChanges(old *model.Route53Record, in recordInput, zone string) ([]r53Types.Change, error) {
	if old != nil && old.AliasTarget != nil {
		return nil, errors.New("editing alias records is not supported")
	}
	name := qualifyRecordName(in.Name, zone)
	rrType := r53Types.RRType(strings.ToUpper(strings.TrimSpace(in.Type)))
	if !slices.Contains(rrType.Values(), rrType) {
		return nil, fmt.Errorf("unknown record type %q", in.Type)
	}
	ttl, err := strconv.ParseInt(strings.TrimSpace(in.TTL), 10, 64)
	if err != nil || ttl < 0 {
		return nil, fmt.Errorf("TTL must be a non-negative number, got %q", in.TTL)
	}
	values := parseRecordValues(in.Values)
	if len(values) == 0 {
		return nil, errors.New("at least one value is required")
	}

	var record r53Types.ResourceRecordSet
	if old != nil {
		record = r53Types.ResourceRecordSet(*old)
	}
	record.Name = aws.String(name)
	record.Type = rrType
	record.TTL = aws.Int64(ttl)
	record.ResourceRecords = values

	if old == nil {
		return []r53Types.Change{{Action: r53Types.ChangeActionCreate, ResourceRecordSet: &record}}, nil
	}
	if sameRecord(r53Types.ResourceRecordSet(*old), record) {
		return nil, nil
	}
	oldRecord := r53Types.ResourceRecordSet(*old)
	return []r53Types.Change{
		{Action: r53Types.ChangeActionDelete, ResourceRecordSet: &oldRecord},
		{Action: r53Types.ChangeActionCreate, ResourceRecordSet: &record},
	}, nil
}

func sameRecord(a, b r53Types.ResourceRecordSet) bool {
	if aws.ToString(a.Name) != aws.ToString(b.Name) || a.Type != b.Type || aws.ToInt64(a.TTL) != aws.ToInt64(b.TTL) {
		return false
	}
	return slices.EqualFunc(a.ResourceRecords, b.ResourceRecords, func(x, y r53Types.ResourceRecord) bool {
		return aws.ToString(x.Value) == aws.ToString(y.Value)
	})
}

func deleteRecordChanges(r model.Route53Record) []r53Types.Change {
	record := r53Types.ResourceRecordSet(r)
	return []r53Types.Change{{Action: r53Types.ChangeActionDelete, ResourceRecordSet: &record}}
}

// describeChanges summarizes changes for the confirmation dialog.
func describeChanges(changes []r53Types.Change) string {
	var lines []string
	for _, c := range changes {
		r := c.ResourceRecordSet
		line := fmt.Sprintf("%v %v %v", c.Action, aws.ToString(r.Name), r.Type)
		if r.SetIdentifier != nil {
			line += fmt.Sprintf(" [%v]", *r.SetIdentifier)
		}
		if r.AliasTarget != nil {
			line += " ALIAS " + aws.ToString(r.AliasTarget.DNSName)
		} else {
			line += fmt.Sprintf(" TTL %v", aws.ToInt64(r.TTL))
			for _, v := range r.ResourceRecords {
				line += "\n    " + aws.ToString(v.Value)
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// Button styles that show the focused button the same way in light and dark
// terminals. tview's defaults depend on the palette, and with some themes the
// unfocused button looks like the selected one.
var (
	buttonStyle          = tcell.StyleDefault
	buttonActivatedStyle = tcell.StyleDefault.Reverse(true).Bold(true)
)

// confirmRecordChanges asks before applying changes to the zone. done is
// called after they have been applied successfully.
func (a *Application) confirmRecordChanges(r53 *repo.Route53, zoneId string, changes []r53Types.Change, done func()) {
	modal := tview.NewModal().
		SetText("Apply these changes?\n\n" + tview.Escape(describeChanges(changes)) + "\n\nTab/←/→ choose, Enter confirm, Esc cancel").
		AddButtons([]string{"Cancel", "Apply"}).
		SetButtonStyle(buttonStyle).
		SetButtonActivatedStyle(buttonActivatedStyle).
		SetDoneFunc(func(_ int, label string) {
			a.pages.RemovePage("confirm")
			a.app.SetFocus(a.pages)
			if label != "Apply" {
				return
			}
			if err := r53.ChangeRecords(a.ctx, zoneId, changes); err != nil {
				a.ShowError(err)
				return
			}
			done()
		})
	modal.SetBackgroundColor(tcell.ColorDefault)
	a.pages.AddPage("confirm", modal, false, true)
	a.app.SetFocus(modal)
}

// showRecordForm lets the user edit a record, or create one if old is nil.
func (a *Application) showRecordForm(r53 *repo.Route53, zoneId, zoneName string, old *model.Route53Record, done func()) {
	var in recordInput
	title := " New record "
	if old != nil {
		if old.AliasTarget != nil {
			a.ShowError(errors.New("editing alias records is not supported"))
			return
		}
		in = recordInputFrom(*old)
		title = " Edit record "
	} else {
		in.Type, in.TTL = string(r53Types.RRTypeA), "300"
	}

	types := make([]string, 0)
	for _, t := range r53Types.RRType("").Values() {
		types = append(types, string(t))
	}
	slices.Sort(types)
	typeIndex := max(slices.Index(types, in.Type), 0)

	form := tview.NewForm()
	form.SetBorder(true).SetTitle(title + "in " + strings.TrimSuffix(zoneName, ".") + " ")
	form.SetButtonStyle(buttonStyle).SetButtonActivatedStyle(buttonActivatedStyle)
	form.AddInputField("Name", in.Name, 50, nil, nil)
	form.AddDropDown("Type", types, typeIndex, nil)
	form.AddInputField("TTL", in.TTL, 10, tview.InputFieldInteger, nil)
	form.AddTextArea("Values", "", 50, 6, 0, nil)
	// AddTextArea puts the cursor at the end, scrolling away the first values
	form.GetFormItemByLabel("Values").(*tview.TextArea).SetText(in.Values, false)
	form.AddButton("Save", func() {
		in.Name = form.GetFormItemByLabel("Name").(*tview.InputField).GetText()
		_, in.Type = form.GetFormItemByLabel("Type").(*tview.DropDown).GetCurrentOption()
		in.TTL = form.GetFormItemByLabel("TTL").(*tview.InputField).GetText()
		in.Values = form.GetFormItemByLabel("Values").(*tview.TextArea).GetText()
		changes, err := buildRecordChanges(old, in, zoneName)
		if err != nil {
			a.ShowError(err)
			return
		}
		closeForm := func() {
			a.pages.RemovePage("form")
			a.app.SetFocus(a.pages)
		}
		if len(changes) == 0 {
			closeForm()
			return
		}
		a.confirmRecordChanges(r53, zoneId, changes, func() {
			closeForm()
			done()
		})
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("form")
		a.app.SetFocus(a.pages)
	})

	centered := tview.NewGrid().
		SetColumns(0, 70, 0).
		SetRows(0, 19, 0).
		AddItem(form, 1, 1, 1, 1, 0, 0, true)
	a.pages.AddPage("form", centered, true, true)
	a.app.SetFocus(form)
}
