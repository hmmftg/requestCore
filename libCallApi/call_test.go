package libCallApi_test

import (
	"encoding/json"
	"testing"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/response"
	"gotest.tools/v3/assert"
)

// SimpleTestData represents a simplified test response structure
type SimpleTestData struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SimpleTestResponse represents a simplified API response
type SimpleTestResponse struct {
	Data   []SimpleTestData `json:"data"`
	Status string           `json:"status"`
	Count  int              `json:"count"`
}

func (s SimpleTestResponse) SetStatus(a int)                {}
func (s SimpleTestResponse) SetHeaders(a map[string]string) {}

func TestCall(t *testing.T) {
	type TestCase struct {
		Name    string
		Request libCallApi.CallParam
		Result  *SimpleTestResponse
		Error   error
	}

	// Create fake API server
	fakeServer := libCallApi.NewFakeAPIServer()
	defer fakeServer.Close()

	callParam := libCallApi.RemoteCallParamData[any, SimpleTestResponse]{
		Api:        libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
		QueryStack: &[]string{"test1", "test2", "test3"},
		Builder: func(status int, rawResp []byte, headers map[string]string) (*SimpleTestResponse, error) {
			var resp SimpleTestResponse
			err := json.Unmarshal(rawResp, &resp)
			if err != nil {
				return nil, response.ToError("", "", err)
			}
			return &resp, nil
		},
	}

	testCases := []TestCase{
		{
			Name: "Test1",
			Result: &SimpleTestResponse{
				Data: []SimpleTestData{
					{ID: 1, Name: "Test Item 1", Value: "Value 1"},
					{ID: 2, Name: "Test Item 2", Value: "Value 2"},
				},
				Status: "success",
				Count:  2,
			},
		},
		{
			Name: "Test2",
			Result: &SimpleTestResponse{
				Data: []SimpleTestData{
					{ID: 3, Name: "Test Item 3", Value: "Value 3"},
					{ID: 4, Name: "Test Item 4", Value: "Value 4"},
				},
				Status: "success",
				Count:  2,
			},
		},
		{
			Name: "Test3",
			Result: &SimpleTestResponse{
				Data: []SimpleTestData{
					{ID: 5, Name: "Test Item 5", Value: "Value 5"},
				},
				Status: "success",
				Count:  1,
			},
		},
	}

	for id := range testCases {
		t.Run(
			testCases[id].Name, func(t *testing.T) {
				result, err := libCallApi.RemoteCall(&callParam)
				assert.DeepEqual(t, err, testCases[id].Error)
				assert.DeepEqual(t, result, testCases[id].Result)
			},
		)
	}
}

func TestCallJSON(t *testing.T) {
	type TestCase struct {
		Name    string
		Request libCallApi.CallParam
		Result  *SimpleTestResponse
		Error   error
	}

	// Create fake API server
	fakeServer := libCallApi.NewFakeAPIServer()
	defer fakeServer.Close()

	callParam := libCallApi.RemoteCallParamData[any, SimpleTestResponse]{
		Api:        libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
		QueryStack: &[]string{"test1", "test2", "test3"},
		Builder: func(status int, rawResp []byte, headers map[string]string) (*SimpleTestResponse, error) {
			var resp SimpleTestResponse
			err := json.Unmarshal(rawResp, &resp)
			if err != nil {
				return nil, response.ToError("", "", err)
			}
			return &resp, nil
		},
	}

	testCases := []TestCase{
		{
			Name: "Test1",
			Result: &SimpleTestResponse{
				Data: []SimpleTestData{
					{ID: 1, Name: "Test Item 1", Value: "Value 1"},
					{ID: 2, Name: "Test Item 2", Value: "Value 2"},
				},
				Status: "success",
				Count:  2,
			},
		},
		{
			Name: "Test2",
			Result: &SimpleTestResponse{
				Data: []SimpleTestData{
					{ID: 3, Name: "Test Item 3", Value: "Value 3"},
					{ID: 4, Name: "Test Item 4", Value: "Value 4"},
				},
				Status: "success",
				Count:  2,
			},
		},
		{
			Name: "Test3",
			Result: &SimpleTestResponse{
				Data: []SimpleTestData{
					{ID: 5, Name: "Test Item 5", Value: "Value 5"},
				},
				Status: "success",
				Count:  1,
			},
		},
	}

	for id := range testCases {
		t.Run(
			testCases[id].Name, func(t *testing.T) {
				result, err := libCallApi.RemoteCall(&callParam)
				assert.DeepEqual(t, err, testCases[id].Error)
				assert.DeepEqual(t, result, testCases[id].Result)
			},
		)
	}
}
