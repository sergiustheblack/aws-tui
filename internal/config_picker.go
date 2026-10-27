package internal

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var awsRegions = []string{
	"af-south-1",
	"ap-east-1",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-northeast-3",
	"ap-south-1",
	"ap-south-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-southeast-3",
	"ap-southeast-4",
	"ap-southeast-5",
	"ca-central-1",
	"ca-west-1",
	"eu-central-1",
	"eu-central-2",
	"eu-north-1",
	"eu-south-1",
	"eu-south-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"il-central-1",
	"me-central-1",
	"me-south-1",
	"sa-east-1",
	"us-east-1",
	"us-east-2",
	"us-gov-east-1",
	"us-gov-west-1",
	"us-west-1",
	"us-west-2",
}

func readAWSProfiles() []string {
	seen := map[string]bool{}
	var profiles []string

	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			profiles = append(profiles, name)
		}
	}

	configFile := os.Getenv("AWS_CONFIG_FILE")
	if configFile == "" {
		configFile = config.DefaultSharedConfigFilename()
	}
	credentialsFile := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credentialsFile == "" {
		credentialsFile = config.DefaultSharedCredentialsFilename()
	}

	parseAWSFile(configFile, func(section string) {
		if section == "default" {
			add("default")
		} else if strings.HasPrefix(section, "profile ") {
			add(strings.TrimPrefix(section, "profile "))
		}
	})

	parseAWSFile(credentialsFile, func(section string) {
		add(section)
	})

	return profiles
}

func parseAWSFile(path string, onSection func(string)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			onSection(line[1 : len(line)-1])
		}
	}
}

func (a *Application) showPicker(title string, items []string, selected string, onSelect func(string)) {
	list := tview.NewList()
	list.ShowSecondaryText(false)
	list.SetBorder(true)

	// typing narrows the list down to the matching items
	var query string
	update := func() {
		if query == "" {
			list.SetTitle(fmt.Sprintf(" %v ", title))
		} else {
			list.SetTitle(fmt.Sprintf(" %v [yellow]%v[-] ", title, tview.Escape(query)))
		}
		list.Clear()
		for _, item := range filterPickerItems(items, query) {
			item := item
			list.AddItem(item, "", 0, func() {
				a.pages.RemovePage("picker")
				a.app.SetFocus(a.pages)
				onSelect(item)
			})
		}
		list.SetCurrentItem(0)
	}
	update()
	for i, item := range items {
		if item == selected {
			list.SetCurrentItem(i)
			break
		}
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			query += string(event.Rune())
			update()
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if q := []rune(query); len(q) > 0 {
				query = string(q[:len(q)-1])
				update()
			}
			return nil
		}
		return event
	})

	height := min(len(items)+2, 20)
	centered := tview.NewGrid().
		SetColumns(0, 60, 0).
		SetRows(0, height, 0).
		AddItem(list, 1, 1, 1, 1, 0, 0, true)

	a.pages.AddPage("picker", centered, true, true)
	a.app.SetFocus(list)
}

// filterPickerItems returns the items containing query, ignoring case. Items
// starting with query come first; otherwise the original order is kept.
func filterPickerItems(items []string, query string) []string {
	query = strings.ToLower(query)
	var prefixed, contained []string
	for _, item := range items {
		lower := strings.ToLower(item)
		if strings.HasPrefix(lower, query) {
			prefixed = append(prefixed, item)
		} else if strings.Contains(lower, query) {
			contained = append(contained, item)
		}
	}
	return append(prefixed, contained...)
}

func (a *Application) showRegionPicker() {
	a.showPicker("Select Region", awsRegions, a.region, func(region string) {
		a.rebuild(a.profile, region)
	})
}

func (a *Application) showProfilePicker() {
	profiles := readAWSProfiles()
	if len(profiles) == 0 {
		a.ShowError(fmt.Errorf("no AWS profiles found in ~/.aws/config or ~/.aws/credentials"))
		return
	}
	selected := a.profile
	if selected == "" {
		selected = "default"
	}
	a.showPicker("Select Profile", profiles, selected, func(profile string) {
		a.rebuild(profile, "")
	})
}

// showSortPicker lets the user pick the column to sort by. Picking the column
// that is already sorted on reverses the direction.
func (a *Application) showSortPicker(s Sortable) {
	const defaultOrder = "(default order)"
	col, desc := s.GetSort()
	columns := s.GetSortColumns()
	items := []string{defaultOrder}
	selected := defaultOrder
	for i, c := range columns {
		if i == col {
			if desc {
				c += " ↓"
			} else {
				c += " ↑"
			}
			selected = c
		}
		items = append(items, c)
	}
	a.showPicker("Sort By", items, selected, func(item string) {
		for i, c := range items {
			if c != item {
				continue
			}
			newCol := i - 1 // account for the default order entry
			s.SetSort(newCol, newCol >= 0 && newCol == col && !desc)
			return
		}
	})
}
