package response

type ResponseHandler interface {
	GetErrorsArray(message string, data any) []ErrorResponse
	HandleErrorState(err error, status int, message string, data any, ctx any)
	Respond(code, status int, message string, data any, abort bool, ctx any)
	RespondWithReceipt(code, status int, message string, data any, printData Receipt, abort bool, ctx any)
}

type InternalError struct {
	Desc    string
	Message any
}

func (e InternalError) Error() string {
	return e.Desc
}
