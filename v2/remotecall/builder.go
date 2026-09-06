package remotecall

import "encoding/json"

// ResponseBuilder converts a RawResponse into a typed response.
//
// Contract:
//   - Builder receives a fully owned RawResponse (caller may retain it).
//   - Builder must not mutate shared state.
//   - Builder errors become ErrorKindDecode.
//   - Builder errors never trip the breaker.
//   - Builder may inspect status, headers, and body.
//   - HTTP 2xx → ResponseBuilder. HTTP non-2xx → RemoteCallError (builder
//     is NOT called for non-2xx).
//   - If callers need typed error bodies for 404/409/etc., they parse
//     RemoteCallError.ResponseBody directly.
type ResponseBuilder[Resp any] interface {
	Build(RawResponse) (*Resp, error)
}

// DefaultBuilder is a ResponseBuilder that JSON-unmarshals the response body
// into a new Resp.
type DefaultBuilder[Resp any] struct{}

// Build unmarshals the RawResponse body as JSON into *Resp.
func (DefaultBuilder[Resp]) Build(raw RawResponse) (*Resp, error) {
	var resp Resp
	if len(raw.Body) == 0 {
		return &resp, nil
	}
	if err := json.Unmarshal(raw.Body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StatusPreservingBuilder is a ResponseBuilder that JSON-unmarshals the
// response body but also preserves the HTTP status code in the response
// if the Resp type implements StatusSetter.
type StatusPreservingBuilder[Resp any] struct{}

// Build unmarshals the RawResponse body as JSON and sets the status code
// if the Resp type implements StatusSetter.
func (StatusPreservingBuilder[Resp]) Build(raw RawResponse) (*Resp, error) {
	var resp Resp
	if len(raw.Body) > 0 {
		if err := json.Unmarshal(raw.Body, &resp); err != nil {
			return nil, err
		}
	}
	if s, ok := any(&resp).(StatusSetter); ok {
		s.SetStatus(raw.StatusCode)
	}
	return &resp, nil
}

// StatusSetter is an optional interface that a Resp type can implement to
// receive the HTTP status code during response building.
type StatusSetter interface {
	SetStatus(code int)
}
