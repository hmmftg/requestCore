package response

import "github.com/hmmftg/requestCore/webFramework"

type ResponseHandler interface {
	OK(w webFramework.WebFramework, resp any)
	OKWithReceipt(w webFramework.WebFramework, resp any, receipt *Receipt)
	OKWithAttachment(w webFramework.WebFramework, file *FileResponse)
	Error(w webFramework.WebFramework, err error)
}

type RespType int

type RespData struct {
	Code       int           `json:"code"`
	Status     int           `json:"status"`
	Message    string        `json:"message"`
	Type       RespType      `json:"type"`
	JSON       any           `json:"description"`
	PrintData  *Receipt      `json:"receipt"`
	Attachment *FileResponse `json:"attachment"`
}

const (
	Json RespType = iota
	JsonWithReceipt
	FileAttachment
)

type WsRemoteResponse struct {
	Status      int             `json:"status"`
	Description string          `json:"description"`
	Result      any             `json:"result,omitempty"`
	ErrorData   []ErrorResponse `json:"errors,omitempty"`
}

type WsResponse struct {
	Status       int      `json:"status"`
	Description  string   `json:"description"`
	Result       any      `json:"result,omitempty"`
	ErrorData    any      `json:"errors,omitempty"`
	PrintReceipt *Receipt `json:"printReceipt,omitempty"`
}

type Receipt struct {
	Id    string `json:"id"`
	Title string `json:"title"`
	Rows  []any  `json:"rows"`
}

type FileResponse struct {
	FileName string `json:"fileName"`
	Path     string `json:"path"`
}

type DbResponse struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
	Result      any    `json:"result"`
	ErrorCode   string `json:"error_code,omitempty"`
}
