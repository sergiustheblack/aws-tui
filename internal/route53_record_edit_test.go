package internal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53 "github.com/aws/aws-sdk-go-v2/service/route53"
	r53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/bporter816/aws-tui/internal/model"
	"github.com/bporter816/aws-tui/internal/repo"
	"github.com/gdamore/tcell/v2"
)

func TestQualifyRecordName(t *testing.T) {
	tests := []struct{ name, expected string }{
		{"www", "www.example.com."},
		{"www.example.com", "www.example.com."},
		{"WWW.Example.COM", "WWW.Example.COM."},
		{"www.example.com.", "www.example.com."},
		{"other.org.", "other.org."},
		{"example.com", "example.com."},
		{"@", "example.com."},
		{"", "example.com."},
		{" a.b ", "a.b.example.com."},
		{"notexample.com", "notexample.com.example.com."},
	}
	for _, test := range tests {
		if got := qualifyRecordName(test.name, "example.com."); got != test.expected {
			t.Errorf("qualifyRecordName(%q): expected %q, got %q", test.name, test.expected, got)
		}
	}
}

func testRecord() model.Route53Record {
	return model.Route53Record{
		Name:            aws.String("www.example.com."),
		Type:            r53Types.RRTypeA,
		TTL:             aws.Int64(300),
		SetIdentifier:   aws.String("blue"),
		Weight:          aws.Int64(10),
		ResourceRecords: []r53Types.ResourceRecord{{Value: aws.String("10.0.0.1")}, {Value: aws.String("10.0.0.2")}},
	}
}

func TestBuildRecordChangesCreate(t *testing.T) {
	changes, err := buildRecordChanges(nil, recordInput{Name: "api", Type: "txt", TTL: "60", Values: "\"a\"\n\n \"b\" \n"}, "example.com.")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Action != r53Types.ChangeActionCreate {
		t.Fatalf("expected one CREATE, got %v", describeChanges(changes))
	}
	r := changes[0].ResourceRecordSet
	if *r.Name != "api.example.com." || r.Type != r53Types.RRTypeTxt || *r.TTL != 60 {
		t.Errorf("unexpected record %v", describeChanges(changes))
	}
	if len(r.ResourceRecords) != 2 || *r.ResourceRecords[0].Value != `"a"` || *r.ResourceRecords[1].Value != `"b"` {
		t.Errorf("unexpected values %v", describeChanges(changes))
	}
}

func TestBuildRecordChangesEdit(t *testing.T) {
	old := testRecord()
	in := recordInputFrom(old)
	in.TTL = "60"
	changes, err := buildRecordChanges(&old, in, "example.com.")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[0].Action != r53Types.ChangeActionDelete || changes[1].Action != r53Types.ChangeActionCreate {
		t.Fatalf("expected DELETE + CREATE, got %v", describeChanges(changes))
	}
	if d := changes[0].ResourceRecordSet; *d.TTL != 300 || len(d.ResourceRecords) != 2 {
		t.Errorf("expected DELETE of the old record, got %v", describeChanges(changes))
	}
	c := changes[1].ResourceRecordSet
	if *c.TTL != 60 || *c.Name != "www.example.com." || len(c.ResourceRecords) != 2 {
		t.Errorf("unexpected new record %v", describeChanges(changes))
	}
	if aws.ToString(c.SetIdentifier) != "blue" || aws.ToInt64(c.Weight) != 10 {
		t.Errorf("expected routing policy to be kept, got %+v", c)
	}
	// the original must not be modified
	if *old.TTL != 300 {
		t.Errorf("old record was modified")
	}
}

func TestBuildRecordChangesUnchanged(t *testing.T) {
	old := testRecord()
	changes, err := buildRecordChanges(&old, recordInputFrom(old), "example.com.")
	if err != nil || len(changes) != 0 {
		t.Errorf("expected no changes, got %v, %v", describeChanges(changes), err)
	}
}

func TestBuildRecordChangesInvalid(t *testing.T) {
	alias := model.Route53Record{Name: aws.String("a.example.com."), Type: r53Types.RRTypeA, AliasTarget: &r53Types.AliasTarget{DNSName: aws.String("x.")}}
	tests := []struct {
		name string
		old  *model.Route53Record
		in   recordInput
	}{
		{"bad type", nil, recordInput{Name: "a", Type: "BOGUS", TTL: "60", Values: "x"}},
		{"bad ttl", nil, recordInput{Name: "a", Type: "A", TTL: "soon", Values: "x"}},
		{"negative ttl", nil, recordInput{Name: "a", Type: "A", TTL: "-1", Values: "x"}},
		{"no values", nil, recordInput{Name: "a", Type: "A", TTL: "60", Values: " \n "}},
		{"alias", &alias, recordInput{Name: "a", Type: "A", TTL: "60", Values: "x"}},
	}
	for _, test := range tests {
		if _, err := buildRecordChanges(test.old, test.in, "example.com."); err == nil {
			t.Errorf("%v: expected an error", test.name)
		}
	}
}

// fakeRoute53 is a local Route 53 endpoint serving one record in zone Z1 and
// recording the change requests it gets. It never reaches AWS.
type fakeRoute53 struct {
	mu      stdsync.Mutex
	changes []string
}

func (f *fakeRoute53) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.changes = append(f.changes, string(body))
		f.mu.Unlock()
		io.WriteString(w, `<ChangeResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ChangeInfo><Id>/change/C1</Id><Status>PENDING</Status><SubmittedAt>2026-01-01T00:00:00Z</SubmittedAt></ChangeInfo></ChangeResourceRecordSetsResponse>`)
		return
	}
	io.WriteString(w, `<ListResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ResourceRecordSets><ResourceRecordSet><Name>www.example.com.</Name><Type>A</Type><TTL>300</TTL><ResourceRecords><ResourceRecord><Value>10.0.0.1</Value></ResourceRecord></ResourceRecords></ResourceRecordSet></ResourceRecordSets><IsTruncated>false</IsTruncated><MaxItems>100</MaxItems></ListResourceRecordSetsResponse>`)
}

func (f *fakeRoute53) getChanges() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.changes...)
}

func newFakeRoute53Repo(t *testing.T, allowWrite bool) (*repo.Route53, *fakeRoute53) {
	t.Helper()
	fake := &fakeRoute53{}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client := r53.New(r53.Options{
		Region:           "us-east-1",
		BaseEndpoint:     aws.String(srv.URL),
		Credentials:      aws.AnonymousCredentials{},
		RetryMaxAttempts: 1,
	})
	return repo.NewRoute53(client, allowWrite), fake
}

func TestRoute53RecordsReadOnlyHasNoEditActions(t *testing.T) {
	r, _ := newFakeRoute53Repo(t, false)
	v := NewRoute53Records(r, "Z1", "example.com.", &Application{})
	if actions := v.GetKeyActions(); len(actions) != 0 {
		t.Errorf("expected no key actions in read-only mode, got %v", actions)
	}
}

func TestRoute53RecordsEdit(t *testing.T) {
	r, fake := newFakeRoute53Repo(t, true)
	a, screen := newTestApplication(t, nil)
	v := NewRoute53Records(r, "Z1", "example.com.", a)
	a.app.QueueUpdate(func() { addComponent(t, a, v) })
	sync(t, a)

	// Name > Type > TTL: replace 300 with 60, then Values > Save
	typeKeys(t, a, screen, "e", tcell.KeyTab, tcell.KeyTab,
		tcell.KeyBackspace2, tcell.KeyBackspace2, tcell.KeyBackspace2, "60",
		tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter)
	if name, _ := a.pages.GetFrontPage(); name != "confirm" {
		t.Fatalf("expected confirmation, got page %q", name)
	}
	if got := fake.getChanges(); len(got) != 0 {
		t.Fatalf("expected no changes before confirming, got %v", got)
	}

	// Cancel is focused first; move to Apply
	typeKeys(t, a, screen, tcell.KeyTab, tcell.KeyEnter)
	got := fake.getChanges()
	if len(got) != 1 {
		t.Fatalf("expected one change request, got %v", got)
	}
	body := got[0]
	del := strings.Index(body, "<Action>DELETE</Action>")
	create := strings.Index(body, "<Action>CREATE</Action>")
	if del < 0 || create < del || !strings.Contains(body[del:create], "<TTL>300</TTL>") || !strings.Contains(body[create:], "<TTL>60</TTL>") {
		t.Errorf("expected DELETE of TTL 300 then CREATE with TTL 60, got %v", body)
	}
	if name, _ := a.pages.GetFrontPage(); name == "form" || name == "confirm" {
		t.Errorf("expected form to be closed, got page %q", name)
	}
}

func TestRoute53RecordsDeleteCancel(t *testing.T) {
	r, fake := newFakeRoute53Repo(t, true)
	a, screen := newTestApplication(t, nil)
	v := NewRoute53Records(r, "Z1", "example.com.", a)
	a.app.QueueUpdate(func() { addComponent(t, a, v) })
	sync(t, a)

	typeKeys(t, a, screen, "D")
	if name, _ := a.pages.GetFrontPage(); name != "confirm" {
		t.Fatalf("expected confirmation, got page %q", name)
	}
	typeKeys(t, a, screen, tcell.KeyEscape)
	typeKeys(t, a, screen, "D", tcell.KeyEnter) // Cancel is the default button
	if got := fake.getChanges(); len(got) != 0 {
		t.Errorf("expected no changes, got %v", got)
	}
	if len(a.components) != 1 {
		t.Errorf("expected to stay in the records view, got %d components", len(a.components))
	}
}
