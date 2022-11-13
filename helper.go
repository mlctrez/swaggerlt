package swaggerlt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

func NewRequestHelper(method, endpoint, uri string) *RequestHelper {
	result := &RequestHelper{
		Endpoint:    endpoint,
		Uri:         uri,
		Method:      strings.ToUpper(method),
		QueryValues: url.Values{},
		PathParam:   map[string]string{},
		Headers:     map[string]string{},
	}
	return result
}

type RequestHelper struct {
	Endpoint    string
	Uri         string
	Method      string
	QueryValues url.Values
	PathParam   map[string]string
	Headers     map[string]string
	Body        any
	Response    any
}

func (u *RequestHelper) Param(name string, value any) {

	switch v := value.(type) {
	case string:
		if v != "" {
			u.QueryValues.Add(name, v)
		}
	case int:
		if v > 0 {
			u.QueryValues.Add(name, strconv.Itoa(v))
		}
	default:
		if reflect.TypeOf(value).Kind() == reflect.Slice {
			s := reflect.ValueOf(value)
			for i := 0; i < s.Len(); i++ {
				sv := fmt.Sprintf("%s", s.Index(i))
				if sv != "" {
					u.QueryValues.Add(name, sv)
				}
			}
		}
	}
}

func (u *RequestHelper) Path(name string, value string) {
	u.PathParam[name] = value
}

func (u *RequestHelper) Header(name string, value string) {
	if value != "" {
		u.Headers[name] = value
	}
}

func (u *RequestHelper) Execute(client *http.Client) error {

	// calculate url from parameters
	uri := u.Uri
	for name, param := range u.PathParam {
		uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", name), param)
	}
	if len(u.QueryValues) > 0 {
		uri += "?" + u.QueryValues.Encode()
	}

	var body io.Reader

	if u.Body != nil {
		marshal, err := json.Marshal(u.Body)
		if err != nil {
			return err
		}
		body = bytes.NewReader(marshal)
	} else {
		body = bytes.NewReader([]byte{})
	}

	request, err := http.NewRequest(u.Method, u.Endpoint+uri, body)
	if err != nil {
		return err
	}

	if u.Body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for header, value := range u.Headers {
		request.Header.Set(header, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	if u.Response != nil {
		err = json.NewDecoder(response.Body).Decode(u.Response)
		if err != nil {
			return err
		}
	} else {
		_, _ = io.ReadAll(request.Body)
		_ = request.Body.Close()
	}
	return nil
}
