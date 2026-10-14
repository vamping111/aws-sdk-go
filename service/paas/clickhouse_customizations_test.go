package paas_test

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/private/protocol/restjson"
	"github.com/aws/aws-sdk-go/service/paas"
)

func TestClickHouseCustomizationLeavesDescribeServiceHandlersUnchanged(t *testing.T) {
	c := client(func(r *http.Request) (*http.Response, error) {
		t.Fatal("handler scope check must not send an HTTP request")
		return nil, nil
	})
	r, _ := c.DescribeServiceRequest(&paas.DescribeServiceInput{ServiceId: aws.String(serviceID)})
	check := func(want request.NamedHandler, seen *bool) func(request.HandlerListRunItem) bool {
		return func(item request.HandlerListRunItem) bool {
			if item.Handler.Name == want.Name {
				*seen = true
				if reflect.ValueOf(item.Handler.Fn).Pointer() != reflect.ValueOf(want.Fn).Pointer() {
					t.Errorf("%s function was replaced", want.Name)
				}
			}
			if strings.HasPrefix(item.Handler.Name, "paas.ClickHouse.") {
				t.Errorf("ClickHouse handler leaked into DescribeService: %s", item.Handler.Name)
			}
			return request.HandlerListStopOnError(item)
		}
	}
	var buildSeen, unmarshalSeen bool
	r.Handlers.Build.AfterEachFn = check(restjson.BuildHandler, &buildSeen)
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}
	r.HTTPResponse = &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}
	r.Handlers.Unmarshal.AfterEachFn = check(restjson.UnmarshalHandler, &unmarshalSeen)
	r.Handlers.Unmarshal.Run(r)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	if !buildSeen || !unmarshalSeen {
		t.Fatalf("original REST JSON handlers missing: build=%t unmarshal=%t", buildSeen, unmarshalSeen)
	}
}
