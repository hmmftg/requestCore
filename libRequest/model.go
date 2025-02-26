package libRequest

import (
	"context"
	"time"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/webFramework"
)

type RequestModel struct {
	QueryInterface libQuery.QueryRunnerInterface
	InsertInDb     string
	UpdateInDb     string
	QueryInDb      string
}

type LoggerInterface interface {
	GetLogPath() string
	GetLogSize() int
	GetLogCompress() bool
	GetSkipPaths() []string
	GetHeaderName() string
}

type RequestInterface interface {
	Initialize(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error)
	InitRequest(c webFramework.WebFramework, method, url string) error
	InitializeNoLog(c webFramework.WebFramework, method, url string, req RequestPtr, args ...any) (int, map[string]string, error)
	InsertRequest(request RequestPtr) error
	CheckDuplicateRequest(request RequestPtr) error
	UpdateRequestWithContext(ctx context.Context, request RequestPtr) error
}

type LogData struct {
	Time    time.Time `json:"dt"`
	Program string    `json:"program"`
	Module  string    `json:"module"`
	Method  string    `json:"method"`
	LogText string    `json:"log_text"`
}

type EventData struct {
	Time time.Time         `json:"dt"`
	Tags map[string]string `json:"tags"`
	Logs []LogData         `json:"logs"`
}

type RequestHeader struct {
	RequestId string `header:"Request-Id" reqHeader:"Request-Id" validate:"required,min=10,max=64"`
	Program   string `header:"Program-Id" reqHeader:"Program-Id"`
	Module    string `header:"Module-Id"  reqHeader:"Module-Id"`
	Method    string `header:"Method-Id"  reqHeader:"Method-Id"`
	User      string `header:"User-Id"    reqHeader:"User-Id"`
	Branch    string `header:"Branch-Id"  reqHeader:"Branch-Id"`
	Bank      string `header:"Bank-Id"    reqHeader:"Bank-Id"`
	Person    string `header:"Person-Id"  reqHeader:"Person-Id"`
}

func (r RequestHeader) GetId() string {
	return r.RequestId
}
func (r RequestHeader) GetUser() string {
	return r.User
}
func (r RequestHeader) GetBank() string {
	return r.Bank
}
func (r RequestHeader) GetBranch() string {
	return r.Branch
}
func (r RequestHeader) GetPerson() string {
	return r.Person
}
func (r RequestHeader) GetProgram() string {
	return r.Program
}
func (r RequestHeader) GetModule() string {
	return r.Module
}
func (r RequestHeader) GetMethod() string {
	return r.Method
}

func (r *RequestHeader) SetUser(user string) {
	r.User = user
}
func (r *RequestHeader) SetProgram(program string) {
	r.Program = program
}
func (r *RequestHeader) SetModule(module string) {
	r.Module = module
}
func (r *RequestHeader) SetMethod(method string) {
	r.Method = method
}

func (r *RequestHeader) SetBranch(branch string) {
	r.Branch = branch
}
func (r *RequestHeader) SetBank(bank string) {
	r.Bank = bank
}
func (r *RequestHeader) SetPerson(person string) {
	r.Person = person
}

type RequestPtr *Request

type Request struct {
	Header    webFramework.HeaderInterface `json:"header"`
	Id        string                       `json:"id"`
	RequestId string                       `json:"request_id"`
	Time      time.Time                    `json:"dt"`
	Incoming  any                          `json:"incoming"`
	Req       string                       `json:"req"`
	Resp      string                       `json:"resp"`
	Outgoing  any                          `json:"outgoing"`
	Tags      map[string]string            `json:"tags"`
	Result    string                       `json:"result"`
}
