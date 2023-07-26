package libRequest

import (
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libValidate"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func GetRequest[Q any](ctx webFramework.WebFramework, isJson bool) (int, string, []response.ErrorResponse, Q, Request, error) {
	return Req[Q, RequestHeader](ctx, isJson)
}

func Req[Req any, Header any, PT interface {
	GetId() string
	GetUser() string
	GetBranch() string
	GetBank() string
	GetPerson() string
	GetProgram() string
	GetModule() string
	GetMethod() string
	SetUser(string)
	SetBranch(string)
	SetBank(string)
	SetPerson(string)
	SetProgram(string)
	SetModule(string)
	SetMethod(string)
	*Header
}](ctx webFramework.WebFramework, isJson bool) (int, string, []response.ErrorResponse, Req, Request, error) {
	var request Req
	var req Request
	var err error

	libValidate.Init()

	// bind the headers to data
	header := PT(new(Header))
	err = ctx.Parser.GetHeader(&header)
	if err != nil {
		return 400, "HEADER_ABSENT", nil, request, req, libError.Join(err, "GetRequest[GetHeader](%v)", ctx.Parser.GetHttpHeader())
	}

	//Check Input JSON
	if isJson {
		err = ctx.Parser.GetBody(&request)
		if err != nil {
			err = libError.Join(err, "GetRequest[GetBody](fails)")
		}
	} else {
		err = ctx.Parser.GetUrlQuery(&request)
		if err != nil {
			err = libError.Join(err, "GetRequest[GetUrlQuery](fails)")
		}
	}
	if err != nil {
		return 400, "JSON_ABSENT", nil, request, req, err
	}

	req = Request{
		Header:   header,
		Id:       header.GetId(),
		Time:     time.Now(),
		Incoming: request, //string(requestJson),
		UserId:   ctx.Parser.GetLocalString("userId"),
		ActionId: ctx.Parser.GetLocalString("action"),
		BranchId: ctx.Parser.GetLocalString("branchId"),
		PersonId: ctx.Parser.GetLocalString("personId"),
		BankId:   ctx.Parser.GetLocalString("bankCode"),
	}
	if len(header.GetBranch()) > 0 {
		req.BranchId = header.GetBranch()
	}
	if len(header.GetBank()) > 0 {
		req.BankId = header.GetBank()
	}
	if len(header.GetPerson()) > 0 {
		req.PersonId = header.GetPerson()
	}

	if ctx.Parser.GetMethod() != "GET" {
		err = libValidate.ValidateStruct(header)
		if err != nil {
			errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
			return 400, "Header Validation Failed", errorResponses, request, req, libError.Join(err, "GetRequest[ValidateHeader](fails)")
		}
	}

	err = libValidate.ValidateStruct(request)
	if err != nil {
		errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
		return 400, "Validation Failed", errorResponses, request, req, libError.Join(err, "GetRequest[ValidateRequest](fails)")
	}

	return 200, "OK", nil, request, req, nil
}

func GetEmptyRequest(ctx webFramework.WebFramework) (int, string, []response.ErrorResponse, Request, error) {
	var req Request
	// bind the headers to data
	var header RequestHeader
	err := ctx.Parser.GetHeader(&header)
	if err != nil {
		return 400, "HEADER_ABSENT", nil, req, err
	}

	req = Request{
		Header:   &header,
		Id:       header.RequestId,
		Time:     time.Now(),
		Incoming: nil,
		UserId:   ctx.Parser.GetLocalString("userId"),
		ActionId: ctx.Parser.GetLocalString("action"),
		BranchId: ctx.Parser.GetLocalString("branchId"),
		BankId:   ctx.Parser.GetLocalString("bankCode"),
	}

	if ctx.Parser.GetMethod() != "GET" {
		libValidate.Init()
		err = libValidate.ValidateStruct(header)
		if err != nil {
			errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
			return 400, "Header Validation Failed", errorResponses, req, err
		}
	}

	return 200, "OK", nil, req, nil
}
