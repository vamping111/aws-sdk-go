package paas

import (
	"encoding/json"
	"io"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/private/protocol/rest"
	"github.com/aws/aws-sdk-go/private/protocol/restjson"
)

func init() {
	initRequest = func(r *request.Request) {
		switch r.Operation.Name {
		case opCreateClickhouseProfile, opModifyClickhouseProfile, opCreateClickhouseQuota, opModifyClickhouseQuota:
			r.Handlers.Build.Swap(restjson.BuildHandler.Name, request.NamedHandler{
				Name: "paas.ClickHouse.Build", Fn: buildClickHouseParameters,
			})
			fallthrough
		case opListClickhouseProfiles, opListClickhouseQuotas:
			r.Handlers.Unmarshal.Swap(restjson.UnmarshalHandler.Name, request.NamedHandler{
				Name: "paas.ClickHouse.Unmarshal", Fn: unmarshalClickHouseParameters,
			})
		}
	}
}

// Profile settings and quota parameters contain arbitrary JSON. Keep their
// encoding local to these operations instead of changing the shared protocol.
func buildClickHouseParameters(r *request.Request) {
	rest.Build(r)
	if r.Error != nil {
		return
	}

	var body map[string]interface{}
	switch p := r.Params.(type) {
	case *CreateClickhouseProfileInput:
		body = clickHouseProfileBody(p.Name, p.Parameters)
	case *ModifyClickhouseProfileInput:
		body = clickHouseProfileBody(nil, p.Parameters)
	case *CreateClickhouseQuotaInput:
		body = clickHouseQuotaBody(p.Name, p.Parameters)
	case *ModifyClickhouseQuotaInput:
		body = clickHouseQuotaBody(nil, p.Parameters)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		r.Error = awserr.New(request.ErrCodeSerialization, "failed encoding ClickHouse parameters", err)
		return
	}
	if r.HTTPRequest.Header.Get("Content-Type") == "" {
		r.HTTPRequest.Header.Set("Content-Type", "application/json")
	}
	r.SetBufferBody(encoded)
}

func clickHouseProfileBody(name *string, parameters *ClickHouseProfileParameters) map[string]interface{} {
	body := map[string]interface{}{}
	if name != nil {
		body["name"] = name
	}
	if parameters != nil {
		values := map[string]interface{}{}
		if parameters.Settings != nil {
			values["settings"] = parameters.Settings
		}
		body["parameters"] = values
	}
	return body
}

func clickHouseQuotaBody(name *string, parameters map[string]interface{}) map[string]interface{} {
	body := map[string]interface{}{}
	if name != nil {
		body["name"] = name
	}
	if parameters != nil {
		body["parameters"] = parameters
	}
	return body
}

func unmarshalClickHouseParameters(r *request.Request) {
	defer r.HTTPResponse.Body.Close()
	if !r.DataFilled() {
		return
	}
	decoder := json.NewDecoder(r.HTTPResponse.Body)
	decoder.UseNumber()
	err := decoder.Decode(r.Data)
	if err == io.EOF {
		return
	}
	if err == nil {
		err = normalizeClickHouseOutput(r.Data)
	}
	if err != nil {
		r.Error = awserr.NewRequestFailure(
			awserr.New(request.ErrCodeSerialization, "failed decoding ClickHouse parameters", err),
			r.HTTPResponse.StatusCode,
			r.RequestID,
		)
	}
}

func normalizeClickHouseOutput(output interface{}) error {
	var values []map[string]interface{}
	addProfile := func(p *ClickHouseProfile) {
		if p != nil && p.Parameters != nil {
			values = append(values, p.Parameters.Settings)
		}
	}
	addQuota := func(q *ClickHouseQuota) {
		if q != nil {
			values = append(values, q.Parameters)
		}
	}
	switch out := output.(type) {
	case *CreateClickhouseProfileOutput:
		addProfile(out.Profile)
	case *ModifyClickhouseProfileOutput:
		addProfile(out.Profile)
	case *ListClickhouseProfilesOutput:
		for _, p := range out.Profiles {
			addProfile(p)
		}
	case *CreateClickhouseQuotaOutput:
		addQuota(out.Quota)
	case *ModifyClickhouseQuotaOutput:
		addQuota(out.Quota)
	case *ListClickhouseQuotasOutput:
		for _, q := range out.Quotas {
			addQuota(q)
		}
	}
	for _, value := range values {
		if err := normalizeClickHouseMap(value); err != nil {
			return err
		}
	}
	return nil
}

// Match the SDK's existing dynamic-map number types: map numbers become
// int64/float64 while values inside free-form arrays remain json.Number.
func normalizeClickHouseMap(values map[string]interface{}) error {
	for key, value := range values {
		switch v := value.(type) {
		case json.Number:
			if integer, err := v.Int64(); err == nil {
				values[key] = integer
			} else {
				decimal, err := v.Float64()
				if err != nil {
					return err
				}
				values[key] = decimal
			}
		case map[string]interface{}:
			if err := normalizeClickHouseMap(v); err != nil {
				return err
			}
		}
	}
	return nil
}
