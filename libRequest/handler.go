package libRequest

import (
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libValidate"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func GetRequest[Q any](ctx webFramework.WebFramework, isJson bool) (int, string, []response.ErrorResponse, Q, Request, error) {
	validateHeader := ctx.Parser.GetMethod() != "GET"
	if isJson {
		return Req[Q, RequestHeader](ctx, JSON, validateHeader)
	}
	return Req[Q, RequestHeader](ctx, Query, validateHeader)
}

//go:generate enumer -type=Type -json -output requestTypeEnum.go
type Type int

const (
	JSON Type = iota
	JSONWithUri
	Query
)

func Req[Req any, Header any, PT interface {
	HeaderInterface
	*Header
}](ctx webFramework.WebFramework, mode Type, validateHeader bool) (int, string, []response.ErrorResponse, Req, Request, error) {
	var request Req
	var req Request
	var err error

	libValidate.Init()

	// bind the headers to data
	header := PT(new(Header))
	err = ctx.Parser.GetHeader(&header)
	if err != nil {
		return http.StatusBadRequest, "HEADER_ABSENT", nil, request, req, libError.Join(err, "GetRequest[GetHeader](%v)", ctx.Parser.GetHttpHeader())
	}

	//Check Input
	desc := "ERROR_IN_GET_REQUEST_"
	if mode == JSON {
		err = ctx.Parser.GetBody(&request)
		if err != nil {
			err = libError.Join(err, "GetRequest[GetBody](fails)")
			desc += "BODY"
		}
	} else if mode == JSONWithUri {
		errBody := ctx.Parser.GetBody(&request)
		if errBody != nil {
			err = libError.Join(errBody, "GetRequest[GetBody](fails)")
			desc += "BODY"
		}
		errUri := ctx.Parser.GetUri(&request)
		if errUri != nil {
			err = libError.Append(err, errUri, "GetRequest[GetUri](fails)")
			desc += "URI"
		}
	} else if mode == Query {
		err = ctx.Parser.GetUrlQuery(&request)
		if err != nil {
			err = libError.Join(err, "GetRequest[GetUrlQuery](fails)")
			desc += "QUERY"
		}
	}
	if err != nil {
		return http.StatusBadRequest, desc, nil, request, req, err
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

	errorResponses := []response.ErrorResponse{}
	if validateHeader {
		err = libValidate.ValidateStruct(header)
		if err != nil {
			errorResponsesHeader := response.FormatErrorResp(err, libValidate.GetTranslator())
			errorResponses = append(errorResponses, errorResponsesHeader...)
		}
	}

	err = libValidate.ValidateStruct(request)
	if err != nil {
		errorResponsesRequest := response.FormatErrorResp(err, libValidate.GetTranslator())
		errorResponses = append(errorResponses, errorResponsesRequest...)
	}
	if len(errorResponses) > 0 {
		return http.StatusBadRequest, "VALIDATION_FAILED", errorResponses, request, req, libError.Join(err, "ParseRequest[ValidateRequest](fails)")
	}

	return http.StatusOK, "OK", nil, request, req, nil
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
