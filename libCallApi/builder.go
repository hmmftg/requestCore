package libCallApi

import (
	"encoding/json"
	"net/http"
)

// StatusPreservingBuilder is a BuilerFunc that preserves the HTTP status code
// in a RemoteCallError when the response is not in the 2xx range. On 2xx it
// unmarshals the JSON body directly (unlike DefaultBuilderfunc which only
// accepts exactly 200). 204 No Content returns a zero Resp without unmarshalling.
//
// Use this builder with RemoteCallParamData when you need to distinguish
// HTTP status codes (401/403/500) in the returned error.
func StatusPreservingBuilder[Resp any](statusCode int, rawResp []byte, headers map[string]string) (*Resp, error) {
	if statusCode < 200 || statusCode >= 300 {
		_, innerErr := DefaultBuilderfunc[Resp](statusCode, rawResp, headers)
		return nil, &RemoteCallError{
			Status: statusCode,
			Body:   rawResp,
			Err:    innerErr,
		}
	}
	if statusCode == http.StatusNoContent || len(rawResp) == 0 {
		return new(Resp), nil
	}
	var resp Resp
	if err := json.Unmarshal(rawResp, &resp); err != nil {
		return nil, &RemoteCallError{
			Status: statusCode,
			Body:   rawResp,
			Err:    err,
		}
	}
	return &resp, nil
}
