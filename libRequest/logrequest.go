package libRequest

import (
	"context"
	"log"
	"net/http"

	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type LogRequest struct {
}

func (l LogRequest) Initialize(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, response.ErrorState) {
	return http.StatusOK, map[string]string{"path": url}, nil
}
func (l LogRequest) InitRequest(c webFramework.WebFramework, method, url string) response.ErrorState {
	return nil
}
func (l LogRequest) InitializeNoLog(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, response.ErrorState) {
	return http.StatusOK, map[string]string{"path": url}, nil

}
func (l LogRequest) AddRequestLog(method, logText string, req RequestPtr) {
	log.Printf("%s - %s(): %s\n", req.Id, method, logText)
}
func (l LogRequest) LogEnd(method, logText string, req RequestPtr) {
	log.Printf("%s - End %s() - log: %s\n", req.Id, method, logText)
}
func (l LogRequest) AddRequestEvent(c webFramework.WebFramework, branch, method, logText string, req RequestPtr) {
	log.Printf("%s - Event %s() - log: %s\n", req.Id, method, logText)
}
func (l LogRequest) LogStart(w webFramework.WebFramework, method, logText string) RequestPtr {
	r := w.Parser.GetLocal("reqLog")
	if r != nil {
		reqLog := r.(RequestPtr)
		branch := w.Parser.GetLocal("branchId")
		if branch == nil {
			branch = ""
		}
		l.AddRequestEvent(w, branch.(string), method, logText, reqLog)
		return reqLog
	}
	return &Request{ActionId: "NONE"}

}
func (l LogRequest) InsertRequest(req RequestPtr) response.ErrorState {
	log.Println("Request Start:", req)
	return nil
}
func (l LogRequest) CheckDuplicateRequest(request RequestPtr) response.ErrorState {
	return nil
}
func (l LogRequest) UpdateRequestWithContext(ctx context.Context, req RequestPtr) response.ErrorState {
	log.Println("Request End:", req)
	return nil
}
