package testingtools

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// initializeTestServer creates a gin server,
// if there was a middleware it will be handled by this function automaticly.
func initializeTestServer(options *TestOptions) *gin.Engine {
	if options.Silent {
		log.SetOutput(io.Discard)
		gin.SetMode(gin.ReleaseMode)
	}

	g := gin.New()

	if options.Middleware != nil {
		g.Use(options.Middleware)
	}

	g.Any(options.Path, libGin.Gin(options.Handler))

	return g
}

// initializeTestServer creates a gin server,
// if there was a middleware it will be handled by this function automaticly.
func initializeTestServerFiber(options *TestOptions) *fiber.App {
	if options.Silent {
		log.SetOutput(io.Discard)
	}

	f := fiber.New()

	if options.Middleware != nil {
		f.Use(options.MiddlewareFiber)
	}

	f.Get(options.Path, libFiber.Fiber(options.Handler))
	f.Post(options.Path, libFiber.Fiber(options.Handler))
	f.Put(options.Path, libFiber.Fiber(options.Handler))
	f.Patch(options.Path, libFiber.Fiber(options.Handler))

	return f
}

// createTestRequest creates a response and a request that can be serve by the test server,
// if request has a body like sending a json to the server it will be marshal to the json,
// and will be send with the created request.
func createTestRequest(t *testing.T, tc *TestCase, method string) (*httptest.ResponseRecorder, *http.Request) {
	bodyJSON, err := json.Marshal(tc.Request)
	if err != nil {
		t.Fatalf("error in marshaling Body: %v", err)
	}

	body := bytes.NewReader(bodyJSON)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, tc.Url, body)

	tc.Header.setHeaders(r)

	return w, r
}

// getTestResponse returns response status code and response body.
func getTestResponse(t *testing.T, w *httptest.ResponseRecorder, tc *TestCase) (int, string, http.Header) {
	res := w.Result()
	defer res.Body.Close()
	return getResponse(t, res, tc)
}

// getTestResponse returns response status code and response body.
func getResponse(t *testing.T, res *http.Response, tc *TestCase) (int, string, http.Header) {
	if !tc.DontReadBody {
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("error in reading from responseWriter: %v", err)
		}
		return res.StatusCode, string(data), res.Header
	}
	return res.StatusCode, "", res.Header
}

// compareTest checks expected status code and expected response
// against the returned response.
func compareTest(t *testing.T, tc *TestCase, status int, response string, responseHeaders http.Header) {
	if tc.Status != status {
		t.Errorf("expected status: %d, got status: %d", tc.Status, status)
	}

	for _, expect := range tc.CheckBody {
		assert.Assert(t, cmp.Contains(response, expect), "Name:%s, Resp: %s", tc.Name, response)
	}

	for expectedKey, expectedHeader := range tc.CheckHeader {
		assert.Assert(t, cmp.Regexp(expectedHeader, responseHeaders[expectedKey][0]), "Name:%s, Resp: %s", tc.Name, response)
	}
}

// doTest runs test for a single test case.
func doTest(t *testing.T, g *gin.Engine, tc *TestCase, options *TestOptions) {
	t.Run(tc.Name, func(t *testing.T) {
		w, r := createTestRequest(t, tc, options.Method)

		g.ServeHTTP(w, r)

		status, response, headers := getTestResponse(t, w, tc)

		compareTest(t, tc, status, response, headers)
	})
}

// doTest runs test for a single test case.
func doTestFiber(t *testing.T, f *fiber.App, tc *TestCase, options *TestOptions) {
	t.Run(tc.Name, func(t *testing.T) {
		_, r := createTestRequest(t, tc, options.Method)

		resp, err := f.Test(r)
		defer func() {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}()
		if err != nil {
			t.Fatal("error execute Test", options.Name, options.Path, err)
		}

		status, response, headers := getResponse(t, resp, tc)

		compareTest(t, tc, status, response, headers)
	})
}

// TestAPI tests a group of Api's test.
func TestAPI(t *testing.T, testCases []TestCase, options *TestOptions) {
	g := initializeTestServer(options)

	for id := range testCases {
		t.Run(testCases[id].Name, func(t *testing.T) {
			doTest(t, g, &testCases[id], options)
		})
	}
}

// TestDB tests a single DB models.
func TestDB(t *testing.T, tc *TestCase, options *TestOptions) {
	g := initializeTestServer(options)

	doTest(t, g, tc, options)
}

// TestDB tests a single DB models.
func TestDBFiber(t *testing.T, tc *TestCase, options *TestOptions) {
	f := initializeTestServerFiber(options)

	doTestFiber(t, f, tc, options)
}
