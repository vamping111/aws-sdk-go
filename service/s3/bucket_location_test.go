package s3_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/unit"
	"github.com/aws/aws-sdk-go/service/s3"
)

var s3LocationTests = []struct {
	body string
	loc  string
}{
	{`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`, ``},
	{`<?xml version="1.0" encoding="UTF-8"?><LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">EU</LocationConstraint>`, `EU`},
}

func TestGetBucketLocation(t *testing.T) {
	for _, test := range s3LocationTests {
		s := s3.New(unit.Session)
		s.Handlers.Send.Clear()
		s.Handlers.Send.PushBack(func(r *request.Request) {
			reader := ioutil.NopCloser(bytes.NewReader([]byte(test.body)))
			r.HTTPResponse = &http.Response{StatusCode: 200, Body: reader}
		})

		resp, err := s.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: aws.String("bucket")})
		if err != nil {
			t.Errorf("expected no error, but received %v", err)
		}

		if test.loc == "" {
			if v := resp.LocationConstraint; v != nil {
				t.Errorf("expect location constraint to be nil, got %s", *v)
			}
		} else {
			if e, a := test.loc, *resp.LocationConstraint; e != a {
				t.Errorf("expect %s location constraint, got %v", e, a)
			}
		}
	}
}

func TestNormalizeBucketLocation(t *testing.T) {
	cases := []struct {
		In, Out string
	}{
		{"", "us-east-1"},
		{"EU", "eu-west-1"},
		{"us-east-1", "us-east-1"},
		{"something", "something"},
	}

	for i, c := range cases {
		actual := s3.NormalizeBucketLocation(c.In)
		if e, a := c.Out, actual; e != a {
			t.Errorf("%d, expect %s bucket location, got %s", i, e, a)
		}
	}
}

func TestWithNormalizeBucketLocation(t *testing.T) {
	req := &request.Request{}
	req.ApplyOptions(s3.WithNormalizeBucketLocation)

	cases := []struct {
		In, Out string
	}{
		{"", "us-east-1"},
		{"EU", "eu-west-1"},
		{"us-east-1", "us-east-1"},
		{"something", "something"},
	}

	for i, c := range cases {
		req.Data = &s3.GetBucketLocationOutput{
			LocationConstraint: aws.String(c.In),
		}
		req.Handlers.Unmarshal.Run(req)

		v := req.Data.(*s3.GetBucketLocationOutput).LocationConstraint
		if e, a := c.Out, aws.StringValue(v); e != a {
			t.Errorf("%d, expect %s bucket location, got %s", i, e, a)
		}
	}
}

func TestLocationConstraintNotPopulatedForAnyRegion(t *testing.T) {
	cases := []struct {
		region string
	}{
		{"us-east-1"},
		{"mock-region"},
	}

	for _, c := range cases {
		t.Run(c.region, func(t *testing.T) {
			s := s3.New(unit.Session, &aws.Config{Region: aws.String(c.region)})

			req, _ := s.CreateBucketRequest(&s3.CreateBucketInput{
				Bucket: aws.String("bucket"),
			})

			if err := req.Build(); err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			v, _ := awsutil.ValuesAtPath(req.Params, "CreateBucketConfiguration.LocationConstraint")
			if l := len(v); l != 0 {
				t.Errorf("expect no values, got %d", l)
			}
		})
	}
}
