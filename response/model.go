package response

import (
	"log/slog"

	"github.com/hmmftg/requestCore/webFramework"
)

// ResponseHandler defines the interface for sending success and error HTTP responses.
type ResponseHandler interface {
	OK(w webFramework.WebFramework, resp any)
	OKWithReceipt(w webFramework.WebFramework, resp any, receipt *Receipt)
	OKWithAttachment(w webFramework.WebFramework, file *FileResponse)
	Error(w webFramework.WebFramework, err error)
}

// RespType identifies the kind of response payload to send.
type RespType int

// RespData holds all fields needed to construct an HTTP response.
type RespData struct {
	Code           int              `json:"code"`
	Status         int              `json:"status"`
	Message        string           `json:"message"`
	Type           RespType         `json:"type"`
	JSON           any              `json:"description"`
	PrintData      *Receipt         `json:"receipt"`
	Attachment     *FileResponse    `json:"attachment"`
	PreBuiltErrors *[]ErrorResponse `json:"-"` // when set, used instead of GetErrorsArray (e.g. for PublicDescription)
}

const (
	// JSON indicates a plain JSON response.
	JSON RespType = iota
	// JSONWithReceipt indicates a JSON response that includes a printable receipt.
	JSONWithReceipt
	// FileAttachment indicates a file-download response.
	FileAttachment
)

// WsRemoteResponse represents the standard structure of a remote API response.
type WsRemoteResponse struct {
	Status      int             `json:"status"`
	Description string          `json:"description"`
	Result      any             `json:"result,omitempty"`
	ErrorData   []ErrorResponse `json:"errors,omitempty"`
}

// WsResponse represents the HTTP response sent to the client.
type WsResponse struct {
	Status       int      `json:"status"`
	Description  string   `json:"description"`
	Result       any      `json:"result,omitempty"`
	ErrorData    any      `json:"errors,omitempty"`
	PrintReceipt *Receipt `json:"printReceipt,omitempty"`
}

// Receipt represents a printable receipt with an ID, title, and data rows.
type Receipt struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Rows  []any  `json:"rows"`
}

// FileResponse holds the file name and path for a file-download response.
type FileResponse struct {
	FileName string `json:"fileName"`
	Path     string `json:"path"`
}

// DbResponse represents the standard structure of a database query response.
type DbResponse struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
	Result      any    `json:"result"`
	ErrorCode   string `json:"error_code,omitempty"`
}

// LogValue returns a structured slog.Value for logging the WsResponse, redacting error details on failure.
func (r WsResponse) LogValue() slog.Value {
	if r.Status == 0 {
		return slog.GroupValue(
			slog.Int("status", r.Status),
			slog.String("description", r.Description),
			slog.Any("result", r.Result),
		)
	}
	return slog.GroupValue(
		slog.Int("status", r.Status),
		slog.String("description", r.Description),
		slog.Any("errorData", r.ErrorData),
	)
}
