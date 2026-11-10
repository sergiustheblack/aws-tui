package internal

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// serviceAliases are extra names for services, keyed by normalized name.
var serviceAliases = map[string]string{
	"ssm":  "Systems Manager",
	"logs": "CloudWatch",
}

// defaultViews are the views opened for services that have several when only
// the service is given.
var defaultViews = map[string]string{
	"EC2":      "Instances",
	"Route 53": "Hosted Zones",
}

// StartView is what to show at startup: a view (registry key) or, if View is
// empty, the services tree with Service expanded.
type StartView struct {
	Service string
	View    string
}

// ResolveStartView turns command line arguments (a service and optionally one
// of its views, e.g. "route53" or "ec2 security groups") into a StartView.
// Names are matched case-insensitively ignoring spaces and punctuation; a
// unique prefix or substring is enough.
func ResolveStartView(args []string) (StartView, error) {
	if len(args) == 0 {
		return StartView{}, nil
	}
	tree := serviceTree()
	var services []string
	for s := range tree {
		services = append(services, s)
	}
	sort.Strings(services)

	service, ok := serviceAliases[normalizeName(args[0])]
	if !ok {
		var err error
		if service, err = matchName("service", args[0], services); err != nil {
			return StartView{}, err
		}
	}
	views := tree[service]

	if len(args) > 1 {
		view, err := matchName(service+" view", strings.Join(args[1:], " "), views)
		if err != nil {
			return StartView{}, err
		}
		return StartView{Service: service, View: service + "." + view}, nil
	}
	if len(views) == 1 {
		return StartView{Service: service, View: service + "." + views[0]}, nil
	}
	if view, ok := defaultViews[service]; ok {
		return StartView{Service: service, View: service + "." + view}, nil
	}
	return StartView{Service: service}, nil
}

// matchName finds name among candidates: an exact match, else the only one it
// is a prefix of, else the only one containing it.
func matchName(kind, name string, candidates []string) (string, error) {
	n := normalizeName(name)
	var prefix, substr []string
	for _, c := range candidates {
		nc := normalizeName(c)
		if n == nc {
			return c, nil
		}
		if strings.HasPrefix(nc, n) {
			prefix = append(prefix, c)
		}
		if strings.Contains(nc, n) {
			substr = append(substr, c)
		}
	}
	for _, m := range [][]string{prefix, substr} {
		if len(m) == 1 {
			return m[0], nil
		}
		if len(m) > 1 {
			return "", fmt.Errorf("ambiguous %v %q, could be: %v", kind, name, strings.Join(m, ", "))
		}
	}
	return "", fmt.Errorf("unknown %v %q, available: %v", kind, name, strings.Join(candidates, ", "))
}

// normalizeName lowercases s and drops everything but letters and digits, so
// "route53", "Route-53" and "Route 53" are the same.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Open shows the start view on top of the services tree.
func (a *Application) Open(start StartView) {
	if start.Service == "" || len(a.components) == 0 {
		return
	}
	s, ok := a.components[0].(*Services)
	if !ok {
		return
	}
	if f, ok := registry[start.View]; ok {
		a.AddAndSwitch(f(s.repos, a))
		return
	}
	s.ExpandService(start.Service)
}
