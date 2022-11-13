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
		Endpoint:      endpoint,
		Uri:           uri,
		Method:        strings.ToUpper(method),
		QueryValues:   url.Values{},
		PathParam:     map[string]string{},
		Headers:       map[string]string{},
		ResponseTypes: map[int]any{},
	}
	return result
}

type RequestHelper struct {
	Endpoint      string
	Uri           string
	Method        string
	QueryValues   url.Values
	PathParam     map[string]string
	Headers       map[string]string
	ResponseTypes map[int]any
	Body          any
	Response      any
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

func (u *RequestHelper) ResponseType(code int, body any) {
	u.ResponseTypes[code] = body
}

func (u *RequestHelper) Execute(client *http.Client) (err error) {

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
		var marshal []byte
		if marshal, err = json.Marshal(u.Body); err != nil {
			return
		}
		body = bytes.NewReader(marshal)
	} else {
		body = bytes.NewBufferString("")
	}

	var request *http.Request
	if request, err = http.NewRequest(u.Method, u.Endpoint+uri, body); err != nil {
		return
	}

	if u.Body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for header, value := range u.Headers {
		request.Header.Set(header, value)
	}

	var response *http.Response
	if response, err = client.Do(request); err != nil {
		return err
	}

	// dispose of the body and close
	defer func() {
		_, _ = io.ReadAll(response.Body)
		_ = request.Body.Close()
	}()

	if response.StatusCode > 299 {
		if rt := u.ResponseTypes[response.StatusCode]; rt == nil {
			return fmt.Errorf("response status code %d with no valid response type", response.StatusCode)
		} else {
			if err = json.NewDecoder(response.Body).Decode(rt); err != nil {
				return
			}
			return &Error{Body: rt, StatusCode: response.StatusCode}
		}
	} else {
		if u.Response != nil {
			err = json.NewDecoder(response.Body).Decode(u.Response)
		} else {
			_, _ = io.ReadAll(request.Body)
		}
	}

	return
}

type Error struct {
	StatusCode int
	Body       any
}

func (r *Error) Error() string {
	marshal, _ := json.Marshal(r.Body)
	return fmt.Sprintf("Error code=%d data=%s", r.StatusCode, string(marshal))
}
