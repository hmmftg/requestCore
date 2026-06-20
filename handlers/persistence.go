package handlers

// RequestPersister optionally persists request lifecycle data for a handler.
// When HandlerParameters.Persistence is nil, insert/update are not called.
//
// Insert must succeed before the handler runs; failure aborts the request.
// Update is best-effort after the response may have been sent; the framework
// logs errors only and does not retry.
type RequestPersister[Req, Resp any] interface {
	Insert(path string, req *HandlerRequest[Req, Resp]) error
	Update(path string, req *HandlerRequest[Req, Resp]) error
}
