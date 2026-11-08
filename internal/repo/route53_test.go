package repo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53 "github.com/aws/aws-sdk-go-v2/service/route53"
	r53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// newTestRoute53 returns a Route53 repo talking to a local server that records
// the requests it gets. It never reaches AWS.
func newTestRoute53(t *testing.T, allowWrite bool) (*Route53, *[]string) {
	t.Helper()
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.Path+"\n"+string(body))
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, `<ChangeResourceRecordSetsResponse xmlns="https://route53.amazonaws.com/doc/2013-04-01/"><ChangeInfo><Id>/change/C1</Id><Status>PENDING</Status><SubmittedAt>2026-01-01T00:00:00Z</SubmittedAt></ChangeInfo></ChangeResourceRecordSetsResponse>`)
	}))
	t.Cleanup(srv.Close)
	client := r53.New(r53.Options{
		Region:           "us-east-1",
		BaseEndpoint:     aws.String(srv.URL),
		Credentials:      aws.AnonymousCredentials{},
		RetryMaxAttempts: 1,
	})
	return NewRoute53(client, allowWrite), &requests
}

var testChanges = []r53Types.Change{{
	Action: r53Types.ChangeActionCreate,
	ResourceRecordSet: &r53Types.ResourceRecordSet{
		Name:            aws.String("www.example.com."),
		Type:            r53Types.RRTypeA,
		TTL:             aws.Int64(60),
		ResourceRecords: []r53Types.ResourceRecord{{Value: aws.String("10.0.0.1")}},
	},
}}

func TestRoute53ChangeRecordsReadOnly(t *testing.T) {
	r, requests := newTestRoute53(t, false)
	if err := r.ChangeRecords(context.Background(), "Z1", testChanges); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
	if len(*requests) != 0 {
		t.Errorf("expected no requests in read-only mode, got %v", *requests)
	}
}

func TestRoute53ChangeRecords(t *testing.T) {
	r, requests := newTestRoute53(t, true)
	if err := r.ChangeRecords(context.Background(), "Z1", testChanges); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one request, got %v", *requests)
	}
	req := (*requests)[0]
	for _, s := range []string{"POST /2013-04-01/hostedzone/Z1/rrset", "<Action>CREATE</Action>", "<Name>www.example.com.</Name>", "<Value>10.0.0.1</Value>"} {
		if !strings.Contains(req, s) {
			t.Errorf("expected request to contain %q, got %v", s, req)
		}
	}
}
