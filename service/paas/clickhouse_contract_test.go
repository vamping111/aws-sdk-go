package paas_test

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/paas"
)

const serviceID = "fm-cluster-12345678"
const profileID = "profile-12345678"
const quotaID = "quota-12345678"
const largestInteger int64 = 9223372036854775807
const settingsJSON = `{"maxInteger":9223372036854775807,"decimal":1.5,"nullable":null,"nested":{"nullable":null}}`
const quotaJSON = `{"interval":[{"duration":3600,"queries":9223372036854775807,"execution_time":1.5,"nullable":null}],"maxInteger":9223372036854775807,"decimal":1.5,"nullable":null,"nested":{"nullable":null}}`
const quotaResponseJSON = `{"interval":[{"duration":3600,"queries":9223372036854775807,"executionTime":1.5,"nullable":null}],"maxInteger":9223372036854775807,"decimal":1.5,"nullable":null,"nested":{"nullable":null}}`
const profileReply = `{"id":"profile-12345678","name":"analytics_profile","parameters":{"settings":` + settingsJSON + `}}`
const quotaReply = `{"id":"quota-12345678","name":"analytics_quota","parameters":` + quotaResponseJSON + `}`
const shardReply = `{"index":0,"parameters":{"internalReplication":false,"replicas":1,"weight":2}}`

type operation struct {
	name, method, path, payload, response string
	input                                 interface{}
	required, uriFields                   []string
}

// The HTTP expectations and required field lists are literal test contracts.
// They are not obtained from request.Operation, SDK tags, or SDK model JSON.
func operations() []operation {
	settings := func() *paas.ClickHouseProfileParameters {
		return &paas.ClickHouseProfileParameters{Settings: map[string]interface{}{"maxInteger": largestInteger, "decimal": 1.5, "nullable": nil, "nested": map[string]interface{}{"nullable": nil}}}
	}
	quota := func() map[string]interface{} {
		return map[string]interface{}{"interval": []interface{}{map[string]interface{}{"duration": int64(3600), "queries": largestInteger, "execution_time": 1.5, "nullable": nil}}, "maxInteger": largestInteger, "decimal": 1.5, "nullable": nil, "nested": map[string]interface{}{"nullable": nil}}
	}
	shard := func() *paas.ClickHouseShardParameters {
		return &paas.ClickHouseShardParameters{InternalReplication: aws.Bool(false), Replicas: aws.Int64(1), Weight: aws.Int64(2)}
	}
	return []operation{
		{"CreateClickhouseProfile", "POST", "/services/fm-cluster-12345678/clickhouse-profiles", `{"name":"analytics_profile","parameters":{"settings":` + settingsJSON + `}}`, `{"profile":` + profileReply + `}`, &paas.CreateClickhouseProfileInput{ServiceId: aws.String(serviceID), Name: aws.String("analytics_profile"), Parameters: settings()}, []string{"ServiceId", "Name", "Parameters"}, []string{"ServiceId"}},
		{"ListClickhouseProfiles", "GET", "/services/fm-cluster-12345678/clickhouse-profiles", "", `{"profiles":[null,` + profileReply + `]}`, &paas.ListClickhouseProfilesInput{ServiceId: aws.String(serviceID)}, []string{"ServiceId"}, []string{"ServiceId"}},
		{"ModifyClickhouseProfile", "PUT", "/services/fm-cluster-12345678/clickhouse-profiles/profile-12345678", `{"parameters":{"settings":` + settingsJSON + `}}`, `{"profile":` + profileReply + `}`, &paas.ModifyClickhouseProfileInput{ServiceId: aws.String(serviceID), ProfileId: aws.String(profileID), Parameters: settings()}, []string{"ServiceId", "ProfileId", "Parameters"}, []string{"ServiceId", "ProfileId"}},
		{"DeleteClickhouseProfile", "DELETE", "/services/fm-cluster-12345678/clickhouse-profiles/profile-12345678", "", "", &paas.DeleteClickhouseProfileInput{ServiceId: aws.String(serviceID), ProfileId: aws.String(profileID)}, []string{"ServiceId", "ProfileId"}, []string{"ServiceId", "ProfileId"}},
		{"CreateClickhouseQuota", "POST", "/services/fm-cluster-12345678/clickhouse-quotas", `{"name":"analytics_quota","parameters":` + quotaJSON + `}`, `{"quota":` + quotaReply + `}`, &paas.CreateClickhouseQuotaInput{ServiceId: aws.String(serviceID), Name: aws.String("analytics_quota"), Parameters: quota()}, []string{"ServiceId", "Name", "Parameters"}, []string{"ServiceId"}},
		{"ListClickhouseQuotas", "GET", "/services/fm-cluster-12345678/clickhouse-quotas", "", `{"quotas":[null,` + quotaReply + `]}`, &paas.ListClickhouseQuotasInput{ServiceId: aws.String(serviceID)}, []string{"ServiceId"}, []string{"ServiceId"}},
		{"ModifyClickhouseQuota", "PUT", "/services/fm-cluster-12345678/clickhouse-quotas/quota-12345678", `{"parameters":` + quotaJSON + `}`, `{"quota":` + quotaReply + `}`, &paas.ModifyClickhouseQuotaInput{ServiceId: aws.String(serviceID), QuotaId: aws.String(quotaID), Parameters: quota()}, []string{"ServiceId", "QuotaId", "Parameters"}, []string{"ServiceId", "QuotaId"}},
		{"DeleteClickhouseQuota", "DELETE", "/services/fm-cluster-12345678/clickhouse-quotas/quota-12345678", "", "", &paas.DeleteClickhouseQuotaInput{ServiceId: aws.String(serviceID), QuotaId: aws.String(quotaID)}, []string{"ServiceId", "QuotaId"}, []string{"ServiceId", "QuotaId"}},
		{"AddClickhouseShard", "POST", "/services/fm-cluster-12345678/clickhouse-shards", `{"parameters":{"internalReplication":false,"replicas":1,"weight":2}}`, `{"shard":` + shardReply + `}`, &paas.AddClickhouseShardInput{ServiceId: aws.String(serviceID), Parameters: shard()}, []string{"ServiceId", "Parameters"}, []string{"ServiceId"}},
		{"ListClickhouseShards", "GET", "/services/fm-cluster-12345678/clickhouse-shards", "", `{"shards":[null,` + shardReply + `]}`, &paas.ListClickhouseShardsInput{ServiceId: aws.String(serviceID)}, []string{"ServiceId"}, []string{"ServiceId"}},
		{"ModifyClickhouseShard", "PUT", "/services/fm-cluster-12345678/clickhouse-shards/0", `{"parameters":{"internalReplication":false,"replicas":1,"weight":2}}`, `{"shard":` + shardReply + `}`, &paas.ModifyClickhouseShardInput{ServiceId: aws.String(serviceID), ShardIndex: aws.Int64(0), Parameters: shard()}, []string{"ServiceId", "ShardIndex", "Parameters"}, []string{"ServiceId"}},
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func client(f transport) *paas.PaaS {
	return paas.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String("ru-msk"), Endpoint: aws.String("https://wirecheck.invalid"), MaxRetries: aws.Int(0),
		Credentials: credentials.NewStaticCredentials("offline-test-access", "offline-test-secret", ""), HTTPClient: &http.Client{Transport: f},
	})))
}

func build(c *paas.PaaS, input interface{}) (*request.Request, interface{}) {
	switch v := input.(type) {
	case *paas.CreateClickhouseProfileInput:
		r, o := c.CreateClickhouseProfileRequest(v)
		return r, o
	case *paas.ListClickhouseProfilesInput:
		r, o := c.ListClickhouseProfilesRequest(v)
		return r, o
	case *paas.ModifyClickhouseProfileInput:
		r, o := c.ModifyClickhouseProfileRequest(v)
		return r, o
	case *paas.DeleteClickhouseProfileInput:
		r, o := c.DeleteClickhouseProfileRequest(v)
		return r, o
	case *paas.CreateClickhouseQuotaInput:
		r, o := c.CreateClickhouseQuotaRequest(v)
		return r, o
	case *paas.ListClickhouseQuotasInput:
		r, o := c.ListClickhouseQuotasRequest(v)
		return r, o
	case *paas.ModifyClickhouseQuotaInput:
		r, o := c.ModifyClickhouseQuotaRequest(v)
		return r, o
	case *paas.DeleteClickhouseQuotaInput:
		r, o := c.DeleteClickhouseQuotaRequest(v)
		return r, o
	case *paas.AddClickhouseShardInput:
		r, o := c.AddClickhouseShardRequest(v)
		return r, o
	case *paas.ListClickhouseShardsInput:
		r, o := c.ListClickhouseShardsRequest(v)
		return r, o
	case *paas.ModifyClickhouseShardInput:
		r, o := c.ModifyClickhouseShardRequest(v)
		return r, o
	default:
		panic("unsupported test input")
	}
}
func invoke(c *paas.PaaS, input interface{}) (interface{}, error) {
	ctx := context.Background()
	switch v := input.(type) {
	case *paas.CreateClickhouseProfileInput:
		return c.CreateClickhouseProfileWithContext(ctx, v)
	case *paas.ListClickhouseProfilesInput:
		return c.ListClickhouseProfilesWithContext(ctx, v)
	case *paas.ModifyClickhouseProfileInput:
		return c.ModifyClickhouseProfileWithContext(ctx, v)
	case *paas.DeleteClickhouseProfileInput:
		return c.DeleteClickhouseProfileWithContext(ctx, v)
	case *paas.CreateClickhouseQuotaInput:
		return c.CreateClickhouseQuotaWithContext(ctx, v)
	case *paas.ListClickhouseQuotasInput:
		return c.ListClickhouseQuotasWithContext(ctx, v)
	case *paas.ModifyClickhouseQuotaInput:
		return c.ModifyClickhouseQuotaWithContext(ctx, v)
	case *paas.DeleteClickhouseQuotaInput:
		return c.DeleteClickhouseQuotaWithContext(ctx, v)
	case *paas.AddClickhouseShardInput:
		return c.AddClickhouseShardWithContext(ctx, v)
	case *paas.ListClickhouseShardsInput:
		return c.ListClickhouseShardsWithContext(ctx, v)
	case *paas.ModifyClickhouseShardInput:
		return c.ModifyClickhouseShardWithContext(ctx, v)
	default:
		panic("unsupported test input")
	}
}
func decoded(t *testing.T, body string) interface{} {
	t.Helper()
	var value interface{}
	d := json.NewDecoder(strings.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return value
}
func parameters(t *testing.T, got map[string]interface{}, want string) {
	t.Helper()
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded(t, string(body)), decoded(t, want)) {
		t.Fatalf("parameters lost values, nulls or precision: %s", body)
	}
	if v, ok := got["maxInteger"].(int64); !ok || v != largestInteger {
		t.Fatalf("max integer must decode as exact int64: %#v", got["maxInteger"])
	}
	if v, ok := got["decimal"].(float64); !ok || v != 1.5 {
		t.Fatalf("decimal must decode as float64: %#v", got["decimal"])
	}
	if v, ok := got["nullable"]; !ok || v != nil {
		t.Fatalf("null map value was lost: %#v", got)
	}
}
func profile(t *testing.T, p *paas.ClickHouseProfile) {
	t.Helper()
	if p == nil || aws.StringValue(p.Id) != profileID || aws.StringValue(p.Name) != "analytics_profile" || p.Parameters == nil {
		t.Fatalf("profile wrapper not decoded: %#v", p)
	}
	parameters(t, p.Parameters.Settings, settingsJSON)
}
func quota(t *testing.T, q *paas.ClickHouseQuota) {
	t.Helper()
	if q == nil || aws.StringValue(q.Id) != quotaID || aws.StringValue(q.Name) != "analytics_quota" {
		t.Fatalf("quota wrapper not decoded: %#v", q)
	}
	parameters(t, q.Parameters, quotaResponseJSON)
	interval := q.Parameters["interval"].([]interface{})[0].(map[string]interface{})
	if v, ok := interval["queries"].(json.Number); !ok || v.String() != "9223372036854775807" {
		t.Fatalf("nested max integer lost precision: %#v", interval)
	}
	if v, ok := interval["executionTime"].(json.Number); !ok || v.String() != "1.5" {
		t.Fatalf("nested decimal lost precision: %#v", interval)
	}
}
func shard(t *testing.T, s *paas.ClickHouseShard) {
	t.Helper()
	if s == nil || s.Index == nil || *s.Index != 0 || s.Parameters == nil || s.Parameters.InternalReplication == nil || *s.Parameters.InternalReplication || aws.Int64Value(s.Parameters.Replicas) != 1 || aws.Int64Value(s.Parameters.Weight) != 2 {
		t.Fatalf("shard wrapper/zero index/false value not decoded: %#v", s)
	}
}
func checkOutput(t *testing.T, out interface{}) {
	t.Helper()
	switch o := out.(type) {
	case *paas.CreateClickhouseProfileOutput:
		profile(t, o.Profile)
	case *paas.ModifyClickhouseProfileOutput:
		profile(t, o.Profile)
	case *paas.ListClickhouseProfilesOutput:
		if len(o.Profiles) != 2 || o.Profiles[0] != nil {
			t.Fatalf("profiles wrapper/null entry: %#v", o)
		}
		profile(t, o.Profiles[1])
	case *paas.DeleteClickhouseProfileOutput:
		if o == nil {
			t.Fatal("nil delete output")
		}
	case *paas.CreateClickhouseQuotaOutput:
		quota(t, o.Quota)
	case *paas.ModifyClickhouseQuotaOutput:
		quota(t, o.Quota)
	case *paas.ListClickhouseQuotasOutput:
		if len(o.Quotas) != 2 || o.Quotas[0] != nil {
			t.Fatalf("quotas wrapper/null entry: %#v", o)
		}
		quota(t, o.Quotas[1])
	case *paas.DeleteClickhouseQuotaOutput:
		if o == nil {
			t.Fatal("nil delete output")
		}
	case *paas.AddClickhouseShardOutput:
		shard(t, o.Shard)
	case *paas.ModifyClickhouseShardOutput:
		shard(t, o.Shard)
	case *paas.ListClickhouseShardsOutput:
		if len(o.Shards) != 2 || o.Shards[0] != nil {
			t.Fatalf("shards wrapper/null entry: %#v", o)
		}
		shard(t, o.Shards[1])
	default:
		t.Fatalf("unexpected response type %T", out)
	}
}

func TestClickHouseWireOperations(t *testing.T) {
	for _, op := range operations() {
		t.Run(op.name, func(t *testing.T) {
			for _, mode := range []string{"builder", "public-method"} {
				t.Run(mode, func(t *testing.T) {
					calls := 0
					c := client(func(r *http.Request) (*http.Response, error) {
						calls++
						if r.Method != op.method || r.URL.EscapedPath() != op.path || r.URL.RawQuery != "" {
							t.Fatalf("wire route got %s %s want %s %s", r.Method, r.URL.String(), op.method, op.path)
						}
						if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
							t.Fatal("request was not SigV4 signed")
						}
						var body []byte
						if r.Body != nil {
							var err error
							body, err = io.ReadAll(r.Body)
							if err != nil {
								t.Fatal(err)
							}
						}
						if op.payload == "" {
							if len(body) != 0 {
								t.Fatalf("bodyless operation sent %s", body)
							}
						} else if !reflect.DeepEqual(decoded(t, string(body)), decoded(t, op.payload)) {
							t.Fatalf("wire payload mismatch: %s", body)
						}
						status := http.StatusOK
						if op.method == http.MethodDelete {
							status = http.StatusNoContent
						}
						return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(op.response)), Request: r}, nil
					})
					var out interface{}
					var err error
					if mode == "builder" {
						var req *request.Request
						req, out = build(c, op.input)
						req.SetContext(context.Background())
						err = req.Send()
					} else {
						out, err = invoke(c, op.input)
					}
					if err != nil {
						t.Fatal(err)
					}
					if calls != 1 {
						t.Fatalf("expected exactly one local transport call, got %d", calls)
					}
					checkOutput(t, out)
				})
			}
		})
	}
}

func changed(input interface{}, field string, empty bool) interface{} {
	old := reflect.ValueOf(input).Elem()
	copy := reflect.New(old.Type())
	copy.Elem().Set(old)
	value := copy.Elem().FieldByName(field)
	if empty {
		value.Set(reflect.ValueOf(aws.String("")))
	} else {
		value.Set(reflect.Zero(value.Type()))
	}
	return copy.Interface()
}
func TestClickHouseRequiredValidation(t *testing.T) {
	for _, op := range operations() {
		t.Run(op.name, func(t *testing.T) {
			c := client(func(r *http.Request) (*http.Response, error) {
				t.Fatal("invalid input reached HTTP transport")
				return nil, nil
			})
			for _, field := range op.required {
				t.Run("missing_"+field, func(t *testing.T) {
					req, _ := build(c, changed(op.input, field, false))
					err := req.Build()
					if err == nil || !strings.Contains(err.Error(), field) {
						t.Fatalf("missing %s must fail SDK request validation, got %v", field, err)
					}
				})
			}
			for _, field := range op.uriFields {
				t.Run("empty_"+field, func(t *testing.T) {
					req, _ := build(c, changed(op.input, field, true))
					err := req.Build()
					if err == nil || !strings.Contains(err.Error(), field) {
						t.Fatalf("empty URI %s must fail SDK request validation, got %v", field, err)
					}
				})
			}
		})
	}
}

func TestRuntimeErrorsReachPublicSDKCall(t *testing.T) {
	for name, value := range map[string]interface{}{"map_NaN": map[string]interface{}{"bad": math.NaN()}, "list_NaN": []interface{}{math.NaN()}, "map_unsupported": map[string]interface{}{"bad": make(chan int)}, "list_unsupported": []interface{}{make(chan int)}} {
		t.Run(name, func(t *testing.T) {
			c := client(func(r *http.Request) (*http.Response, error) {
				t.Fatal("invalid JSON value reached HTTP transport")
				return nil, nil
			})
			_, err := c.ModifyClickhouseQuotaWithContext(context.Background(), &paas.ModifyClickhouseQuotaInput{ServiceId: aws.String(serviceID), QuotaId: aws.String(quotaID), Parameters: map[string]interface{}{"nested": value}})
			if err == nil || !strings.Contains(err.Error(), "SerializationError") {
				t.Fatalf("collection serialization error was swallowed: %v", err)
			}
		})
	}
	for name, body := range map[string]string{"flat_overflow": `{"profiles":[{"parameters":{"settings":{"limit":1e999}}}]}`, "nested_overflow": `{"profiles":[{"parameters":{"settings":{"nested":{"limit":1e999}}}}]}`} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			c := client(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			_, err := c.ListClickhouseProfilesWithContext(context.Background(), &paas.ListClickhouseProfilesInput{ServiceId: aws.String(serviceID)})
			if calls != 1 || err == nil || !strings.Contains(err.Error(), "value out of range") {
				t.Fatalf("JSON map conversion error was swallowed: calls=%d err=%v", calls, err)
			}
		})
	}
}
