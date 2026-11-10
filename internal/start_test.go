package internal

import (
	"strings"
	"testing"
)

func TestResolveStartView(t *testing.T) {
	tests := []struct {
		args []string
		want StartView
	}{
		{nil, StartView{}},
		{[]string{"s3"}, StartView{"S3", "S3.Buckets"}},
		{[]string{"route53"}, StartView{"Route 53", "Route 53.Hosted Zones"}},
		{[]string{"Route-53", "health"}, StartView{"Route 53", "Route 53.Health Checks"}},
		{[]string{"ec2"}, StartView{"EC2", "EC2.Instances"}},
		{[]string{"ec2", "security", "groups"}, StartView{"EC2", "EC2.Security Groups"}},
		{[]string{"ssm"}, StartView{"Systems Manager", "Systems Manager.Parameters"}},
		{[]string{"dynamo"}, StartView{"DynamoDB", "DynamoDB.Tables"}},
		{[]string{"iam"}, StartView{"IAM", ""}},
		{[]string{"IAM", "roles"}, StartView{"IAM", "IAM.Roles"}},
	}
	for _, tt := range tests {
		got, err := ResolveStartView(tt.args)
		if err != nil {
			t.Errorf("%q: %v", tt.args, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q: got %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestResolveStartViewErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"nope"}, "unknown service"},
		{[]string{"e"}, "ambiguous service"},
		{[]string{"ec2", "nope"}, "unknown EC2 view"},
	}
	for _, tt := range tests {
		_, err := ResolveStartView(tt.args)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: got error %v, want %q", tt.args, err, tt.want)
		}
	}
}

func TestServicesExpandService(t *testing.T) {
	s := NewServices(Repos{}, nil)
	s.ExpandService("IAM")
	n := s.GetCurrentNode()
	if n == nil || n.GetReference() != "IAM.Groups" {
		t.Fatalf("selected %v, want IAM.Groups", n)
	}
}
