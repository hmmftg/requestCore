package handlers

import "github.com/hmmftg/requestCore/webFramework"

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

const PersistedRecordIDKey = "handlers.persisted_record_id"

func SetPersistedRecordID(w webFramework.WebFramework, id any) {
	w.Parser.SetLocal(PersistedRecordIDKey, id)
}

func GetPersistedRecordID(w webFramework.WebFramework) (any, bool) {
	v := w.Parser.GetLocal(PersistedRecordIDKey)
	if v == nil {
		return nil, false
	}
	return v, true
}

type FuncPersister[Req, Resp any] struct {
	InsertFn func(path string, req *HandlerRequest[Req, Resp]) error
	UpdateFn func(path string, req *HandlerRequest[Req, Resp]) error
}

func (p FuncPersister[Req, Resp]) Insert(path string, req *HandlerRequest[Req, Resp]) error {
	if p.InsertFn == nil {
		return nil
	}
	return p.InsertFn(path, req)
}

func (p FuncPersister[Req, Resp]) Update(path string, req *HandlerRequest[Req, Resp]) error {
	if p.UpdateFn == nil {
		return nil
	}
	return p.UpdateFn(path, req)
}
