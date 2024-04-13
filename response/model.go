package response

import (
	"github.com/hmmftg/requestCore/webFramework"
)

type ResponseHandler interface {
	GetErrorsArray(message string, data any) []ErrorResponse
	HandleErrorState(err error, status int, message string, data any, w webFramework.WebFramework)
	Respond(data RespData, abort bool, w webFramework.WebFramework)
	OK(w webFramework.WebFramework, resp any)
	OKWithReceipt(w webFramework.WebFramework, resp any, receipt *Receipt)
	OKWithAttachment(w webFramework.WebFramework, file *FileResponse)
	Error(w webFramework.WebFramework, err ErrorState)
}

type RespType int

type RespData struct {
	Code       int
	Status     int
	Message    string
	Type       RespType
	JSON       any
	PrintData  *Receipt
	Attachment *FileResponse
}

const (
	Json RespType = iota
	JsonWithReceipt
	FileAttachment
)

type InternalError struct {
	Desc    string
	Message any
}

func (e InternalError) Error() string {
	return e.Desc
}
