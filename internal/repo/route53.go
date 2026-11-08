package repo

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	r53 "github.com/aws/aws-sdk-go-v2/service/route53"
	r53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/bporter816/aws-tui/internal/model"
	"strings"
)

// ErrReadOnly is returned by calls that would change AWS resources when the
// app was started without write access.
var ErrReadOnly = errors.New("read-only mode: restart with --allow-write to make changes")

type Route53 struct {
	r53Client  *r53.Client
	allowWrite bool
}

func NewRoute53(r53Client *r53.Client, allowWrite bool) *Route53 {
	return &Route53{
		r53Client:  r53Client,
		allowWrite: allowWrite,
	}
}

func (r Route53) AllowWrite() bool {
	return r.allowWrite
}

// ChangeRecords applies changes to the records of a hosted zone as a single
// batch: either all of them succeed or none does.
func (r Route53) ChangeRecords(ctx context.Context, hostedZoneId string, changes []r53Types.Change) error {
	if !r.allowWrite {
		return ErrReadOnly
	}
	if len(changes) == 0 {
		return nil
	}
	_, err := r.r53Client.ChangeResourceRecordSets(
		ctx,
		&r53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(hostedZoneId),
			ChangeBatch:  &r53Types.ChangeBatch{Changes: changes},
		},
	)
	return err
}

func (r Route53) ListHostedZones(ctx context.Context) ([]model.Route53HostedZone, error) {
	pg := r53.NewListHostedZonesPaginator(
		r.r53Client,
		&r53.ListHostedZonesInput{},
	)
	var hostedZones []model.Route53HostedZone
	for pg.HasMorePages() {
		out, err := pg.NextPage(ctx)
		if err != nil {
			return []model.Route53HostedZone{}, err
		}
		for _, v := range out.HostedZones {
			hostedZones = append(hostedZones, model.Route53HostedZone(v))
		}
	}
	return hostedZones, nil
}

func (r Route53) ListHealthChecks(ctx context.Context) ([]model.Route53HealthCheck, error) {
	pg := r53.NewListHealthChecksPaginator(
		r.r53Client,
		&r53.ListHealthChecksInput{},
	)
	var healthChecks []model.Route53HealthCheck
	for pg.HasMorePages() {
		out, err := pg.NextPage(ctx)
		if err != nil {
			return []model.Route53HealthCheck{}, err
		}
		for _, v := range out.HealthChecks {
			healthChecks = append(healthChecks, model.Route53HealthCheck(v))
		}
	}
	return healthChecks, nil
}

func (r Route53) ListRecords(ctx context.Context, hostedZoneId string) ([]model.Route53Record, error) {
	// ListResourceRecordSets doesn't have a paginator :'(
	good := true
	var resourceRecordSets []model.Route53Record
	var nextRecordName *string = nil
	var nextRecordType r53Types.RRType
	var nextRecordIdentifier *string = nil
	for good {
		out, err := r.r53Client.ListResourceRecordSets(
			ctx,
			&r53.ListResourceRecordSetsInput{
				HostedZoneId:          aws.String(hostedZoneId),
				StartRecordName:       nextRecordName,
				StartRecordType:       nextRecordType,
				StartRecordIdentifier: nextRecordIdentifier,
			},
		)
		if err != nil {
			return []model.Route53Record{}, err
		}
		for _, v := range out.ResourceRecordSets {
			resourceRecordSets = append(resourceRecordSets, model.Route53Record(v))
		}
		good = out.IsTruncated
		if out.IsTruncated {
			nextRecordName = out.NextRecordName
			nextRecordType = out.NextRecordType
			nextRecordIdentifier = out.NextRecordIdentifier
		}
	}
	return resourceRecordSets, nil
}

func (r Route53) ListTags(ctx context.Context, typeAndName string) (model.Tags, error) {
	parts := strings.Split(typeAndName, ":")
	if len(parts) != 2 {
		return model.Tags{}, errors.New("must specify resource type and id for route53 tags")
	}
	var resourceType r53Types.TagResourceType
	switch parts[0] {
	case string(r53Types.TagResourceTypeHostedzone):
		resourceType = r53Types.TagResourceTypeHostedzone
	case string(r53Types.TagResourceTypeHealthcheck):
		resourceType = r53Types.TagResourceTypeHealthcheck
	}
	out, err := r.r53Client.ListTagsForResource(
		ctx,
		&r53.ListTagsForResourceInput{
			ResourceId:   aws.String(parts[1]),
			ResourceType: resourceType,
		},
	)
	if err != nil || out.ResourceTagSet == nil {
		return model.Tags{}, err
	}
	var tags model.Tags
	for _, v := range out.ResourceTagSet.Tags {
		tags = append(tags, model.Tag{Key: *v.Key, Value: *v.Value})
	}
	return tags, nil
}
