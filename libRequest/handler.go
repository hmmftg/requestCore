package libRequest

import (
	"log"
	"time"

	"github.com/hmmftg/requestCore/libValidate"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func GetRequest[Q any](ctx webFramework.WebFramework, isJson bool) (int, string, []response.ErrorResponse, Q, Request, error) {
	var request Q
	var req Request
	var err error

	libValidate.Init()

	// bind the headers to data
	var header RequestHeader
	err = ctx.Parser.GetHeader(&header)
	if err != nil {
		log.Println(ctx.Parser.GetHttpHeader())
		return 400, "HEADER_ABSENT", nil, request, req, err
	}

	//Check Input JSON
	if isJson {
		err = ctx.Parser.GetBody(&request)
	} else {
		err = ctx.Parser.GetUrlQuery(&request)
	}
	if err != nil {
		return 400, "JSON_ABSENT", nil, request, req, err
	}

	req = Request{
		Header:   &header,
		Id:       header.RequestId,
		Time:     time.Now(),
		Incoming: request, //string(requestJson),
		UserId:   ctx.Parser.GetLocalString("userId"),
		ActionId: ctx.Parser.GetLocalString("action"),
		BranchId: ctx.Parser.GetLocalString("branchId"),
		BankId:   ctx.Parser.GetLocalString("bankCode"),
	}

	if ctx.Parser.GetMethod() != "GET" {
		err = libValidate.ValidateStruct(header)
		if err != nil {
			errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
			return 400, "Header Validation Failed", errorResponses, request, req, err
		}
	}

	err = libValidate.ValidateStruct(request)
	if err != nil {
		errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
		return 400, "Validation Failed", errorResponses, request, req, err
	}

	return 200, "OK", nil, request, req, nil
}

func Req[Req any, Header HeaderInterface](ctx webFramework.WebFramework, isJson bool) (int, string, []response.ErrorResponse, Req, Request, error) {
	var request Req
	var req Request
	var err error

	libValidate.Init()

	// bind the headers to data
	var header Header
	err = ctx.Parser.GetHeader(&header)
	if err != nil {
		log.Println(ctx.Parser.GetHttpHeader())
		return 400, "HEADER_ABSENT", nil, request, req, err
	}

	//Check Input JSON
	if isJson {
		err = ctx.Parser.GetBody(&request)
	} else {
		err = ctx.Parser.GetUrlQuery(&request)
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
		BankId:   ctx.Parser.GetLocalString("bankCode"),
	}

	if ctx.Parser.GetMethod() != "GET" {
		err = libValidate.ValidateStruct(header)
		if err != nil {
			errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
			return 400, "Header Validation Failed", errorResponses, request, req, err
		}
	}

	err = libValidate.ValidateStruct(request)
	if err != nil {
		errorResponses := response.FormatErrorResp(err, libValidate.GetTranslator())
		return 400, "Validation Failed", errorResponses, request, req, err
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
