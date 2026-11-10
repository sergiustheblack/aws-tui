package repo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeS3 serves buckets "near" (us-east-1, same as the client) and "far"
// (eu-west-1) and records the signing region of each request.
type fakeS3 struct {
	mu      sync.Mutex
	regions map[string]string // "METHOD /path" -> signing region
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	region := ""
	if i := strings.Index(auth, "Credential="); i >= 0 {
		region = strings.Split(auth[i:], "/")[2]
	}
	f.mu.Lock()
	f.regions[r.Method+" "+r.URL.Path] = region
	f.mu.Unlock()
	if r.URL.Path == "/" {
		w.Write([]byte(`<ListAllMyBucketsResult><Buckets>` +
			`<Bucket><Name>near</Name><BucketRegion>us-east-1</BucketRegion></Bucket>` +
			`<Bucket><Name>far</Name><BucketRegion>eu-west-1</BucketRegion></Bucket>` +
			`</Buckets></ListAllMyBucketsResult>`))
		return
	}
	bucket := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")[0]
	want := map[string]string{"near": "us-east-1", "far": "eu-west-1", "unlisted": "eu-west-1"}[bucket]
	w.Header().Set("X-Amz-Bucket-Region", want)
	if region != want {
		w.WriteHeader(http.StatusMovedPermanently)
		if r.Method != http.MethodHead {
			w.Write([]byte(`<Error><Code>PermanentRedirect</Code></Error>`))
		}
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	w.Write([]byte(`<ListBucketResult><Contents><Key>a.txt</Key></Contents></ListBucketResult>`))
}

func newFakeS3Repo(t *testing.T) (*S3, *fakeS3) {
	t.Helper()
	fake := &fakeS3{regions: map[string]string{}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	client := s3.New(s3.Options{
		Region:           "us-east-1",
		BaseEndpoint:     aws.String(srv.URL),
		UsePathStyle:     true,
		Credentials:      credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		RetryMaxAttempts: 1,
	})
	return NewS3(client), fake
}

func TestS3ListObjectsUsesBucketRegion(t *testing.T) {
	r, fake := newFakeS3Repo(t)
	ctx := context.Background()
	if _, err := r.ListBuckets(ctx); err != nil {
		t.Fatal(err)
	}
	for _, bucket := range []string{"near", "far"} {
		_, objects, err := r.ListObjects(ctx, bucket, "")
		if err != nil {
			t.Fatalf("%s: %v", bucket, err)
		}
		if len(objects) != 1 {
			t.Errorf("%s: got objects %v", bucket, objects)
		}
	}
	if _, ok := fake.regions["HEAD /far"]; ok {
		t.Error("region known from ListBuckets, HeadBucket should not be called")
	}
}

func TestS3ListObjectsLooksUpUnknownBucketRegion(t *testing.T) {
	r, fake := newFakeS3Repo(t)
	_, objects, err := r.ListObjects(context.Background(), "unlisted", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 {
		t.Errorf("got objects %v", objects)
	}
	if got := fake.regions["GET /unlisted"]; got != "eu-west-1" {
		t.Errorf("ListObjectsV2 signed for %q, want eu-west-1", got)
	}
}
