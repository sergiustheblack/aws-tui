package repo

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/bporter816/aws-tui/internal/model"
)

type S3 struct {
	s3Client *s3.Client
	// bucket name -> region; requests for a bucket in another region than the
	// client's fail with 301 PermanentRedirect, so they are sent there instead
	regions *sync.Map
}

func NewS3(s3Client *s3.Client) *S3 {
	return &S3{
		s3Client: s3Client,
		regions:  &sync.Map{},
	}
}

func (s S3) ListBuckets(ctx context.Context) ([]model.S3Bucket, error) {
	// BucketRegion is only returned when the request has a parameter
	pg := s3.NewListBucketsPaginator(
		s.s3Client,
		&s3.ListBucketsInput{MaxBuckets: aws.Int32(10000)},
	)
	var buckets []model.S3Bucket
	for pg.HasMorePages() {
		out, err := pg.NextPage(ctx)
		if err != nil {
			return []model.S3Bucket{}, err
		}
		for _, v := range out.Buckets {
			if v.Name != nil && v.BucketRegion != nil && *v.BucketRegion != "" {
				s.regions.Store(*v.Name, *v.BucketRegion)
			}
			buckets = append(buckets, model.S3Bucket(v))
		}
	}
	return buckets, nil
}

// inRegion returns an option that sends a request for the bucket to the
// bucket's region, looking it up with HeadBucket if it isn't known yet.
func (s S3) inRegion(ctx context.Context, bucketName string) func(*s3.Options) {
	region, ok := s.regions.Load(bucketName)
	if !ok {
		if r := s.lookupRegion(ctx, bucketName); r != "" {
			s.regions.Store(bucketName, r)
			region, ok = r, true
		}
	}
	return func(o *s3.Options) {
		if ok {
			o.Region = region.(string)
		}
	}
}

// lookupRegion reads the bucket region that S3 reports in a header even when
// HeadBucket fails (redirect, access denied). Returns "" if unknown.
func (s S3) lookupRegion(ctx context.Context, bucketName string) string {
	out, err := s.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucketName)})
	if err == nil {
		return aws.ToString(out.BucketRegion)
	}
	var re interface{ HTTPResponse() *smithyhttp.Response }
	if errors.As(err, &re) && re.HTTPResponse() != nil {
		return re.HTTPResponse().Header.Get("X-Amz-Bucket-Region")
	}
	return ""
}

func (s S3) ListObjects(ctx context.Context, bucketName string, prefix string) ([]string, []string, error) {
	pg := s3.NewListObjectsV2Paginator(
		s.s3Client,
		&s3.ListObjectsV2Input{
			Bucket:    aws.String(bucketName),
			Delimiter: aws.String("/"),
			Prefix:    aws.String(prefix),
		},
	)
	inRegion := s.inRegion(ctx, bucketName)
	var prefixes, objects []string
	for pg.HasMorePages() {
		out, err := pg.NextPage(ctx, inRegion)
		if err != nil {
			return []string{}, []string{}, err
		}
		for _, v := range out.CommonPrefixes {
			prefixes = append(prefixes, *v.Prefix)
		}
		for _, v := range out.Contents {
			objects = append(objects, *v.Key)
		}
	}
	return prefixes, objects, nil
}

func (s S3) GetBucketPolicy(ctx context.Context, bucketName string) (string, error) {
	out, err := s.s3Client.GetBucketPolicy(
		ctx,
		&s3.GetBucketPolicyInput{
			Bucket: aws.String(bucketName),
		},
		s.inRegion(ctx, bucketName),
	)
	if err != nil || out.Policy == nil {
		return "", err
	}
	return *out.Policy, nil
}

func (s S3) GetCORSRules(ctx context.Context, bucketName string) ([]model.S3CORSRule, error) {
	out, err := s.s3Client.GetBucketCors(
		ctx,
		&s3.GetBucketCorsInput{
			Bucket: aws.String(bucketName),
		},
		s.inRegion(ctx, bucketName),
	)
	if err != nil {
		return []model.S3CORSRule{}, err
	}
	var corsRules []model.S3CORSRule
	for _, v := range out.CORSRules {
		corsRules = append(corsRules, model.S3CORSRule(v))
	}
	return corsRules, nil
}

func (s S3) listBucketTags(ctx context.Context, bucketName string) (model.Tags, error) {
	// TODO find where the panic occurs when there are no tags
	out, err := s.s3Client.GetBucketTagging(
		ctx,
		&s3.GetBucketTaggingInput{
			Bucket: aws.String(bucketName),
		},
		s.inRegion(ctx, bucketName),
	)
	if err != nil {
		return model.Tags{}, err
	}
	var tags model.Tags
	for _, v := range out.TagSet {
		tags = append(tags, model.Tag{Key: *v.Key, Value: *v.Value})
	}
	return tags, nil
}

func (s S3) GetObject(ctx context.Context, bucketName string, key string) ([]byte, error) {
	out, err := s.s3Client.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		},
		s.inRegion(ctx, bucketName),
	)
	if err != nil {
		return []byte{}, err
	}
	defer out.Body.Close()
	if out.ContentLength == nil {
		return []byte{}, errors.New("missing content length")
	}
	b, err := io.ReadAll(out.Body)
	if err != nil {
		return []byte{}, err
	}
	return b, nil
}

func (s S3) GetObjectMetadata(ctx context.Context, bucketName string, key string) (model.Tags, error) {
	out, err := s.s3Client.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		},
		s.inRegion(ctx, bucketName),
	)
	if err != nil {
		return model.Tags{}, err
	}
	var tags model.Tags
	if out.ContentType != nil {
		tags = append(tags, model.Tag{Key: "Content-Type", Value: *out.ContentType})
	}
	for k, v := range out.Metadata {
		tags = append(tags, model.Tag{Key: k, Value: v})
	}
	return tags, nil
}

func (s S3) listObjectTags(ctx context.Context, bucketName string, key string) (model.Tags, error) {
	out, err := s.s3Client.GetObjectTagging(
		ctx,
		&s3.GetObjectTaggingInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		},
		s.inRegion(ctx, bucketName),
	)
	if err != nil {
		return model.Tags{}, err
	}
	var tags model.Tags
	for _, v := range out.TagSet {
		tags = append(tags, model.Tag{Key: *v.Key, Value: *v.Value})
	}
	return tags, nil
}

func (s S3) ListTags(ctx context.Context, typeAndName string) (model.Tags, error) {
	parts := strings.Split(typeAndName, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return model.Tags{}, errors.New("must give type and name for s3 tags")
	}
	switch parts[0] {
	case "bucket":
		return s.listBucketTags(ctx, parts[1])
	case "object":
		return s.listObjectTags(ctx, parts[1], parts[2])
	default:
		return model.Tags{}, errors.New("must use bucket or object for s3 tags")
	}
}
