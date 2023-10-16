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
	NoBinding Type = iota
	JSON
	JSONWithURI
	Query
	QueryWithURI
	URI
)

func parseRequest[Req any](w webFramework.WebFramework, mode Type, validateHeader bool, header webFramework.HeaderInterface, name string) (*Req, RequestPtr, response.Err, []response.ErrorResponse) {
	libValidate.Init()
	var err error
	var request Req

	//Check Input
	desc := "ERROR_IN_GET_REQUEST_"
	switch mode {
	case JSON:
		err = w.Parser.GetBody(&request)
		if err != nil {
			err = libError.Join(err, "%s[GetBody](fails)", name)
			desc += "BODY"
		}
	case JSONWithURI:
		err = w.Parser.GetBody(&request)
		if err != nil {
			err = libError.Join(err, "%s[GetBody](fails)", name)
			desc += "BODY"
		}
		errUri := w.Parser.GetUri(&request)
		if errUri != nil {
			err = libError.Append(err, errUri, "%s[GetUri](fails)", name)
			desc += "URI"
		}
	case Query:
		err = w.Parser.GetUrlQuery(&request)
		if err != nil {
			err = libError.Join(err, "%s[GetUrlQuery](fails)", name)
			desc += "QUERY"
		}
	case QueryWithURI:
		err = w.Parser.GetUrlQuery(&request)
		if err != nil {
			err = libError.Join(err, "%s[GetUrlQuery](fails)", name)
			desc += "QUERY"
		}
		errUri := w.Parser.GetUri(&request)
		if errUri != nil {
			err = libError.Append(err, errUri, "%s[GetUri](fails)", name)
			desc += "URI"
		}
	case URI:
		err = w.Parser.GetUri(&request)
		if err != nil {
			err = libError.Join(err, "%s[GetUri](fails)", name)
			desc += "URI"
		}
	default:
		err = nil
	}
	if err != nil {
		return nil, nil, response.Error(http.StatusBadRequest, desc, nil, err), nil
	}

	req := Request{
		Header:   header,
		Id:       header.GetId(),
		Time:     time.Now(),
		Incoming: request, //string(requestJson),
		UserId:   w.Parser.GetLocalString("userId"),
		ActionId: w.Parser.GetLocalString("action"),
		BranchId: w.Parser.GetLocalString("branchId"),
		PersonId: w.Parser.GetLocalString("personId"),
		BankId:   w.Parser.GetLocalString("bankCode"),
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
		errValidate := libValidate.ValidateStruct(header)
		if errValidate != nil {
			errorResponsesHeader := response.FormatErrorResp(errValidate, libValidate.GetTranslator())
			errorResponses = append(errorResponses, errorResponsesHeader...)
		}
	}

	if mode != NoBinding {
		errValidate := libValidate.ValidateStruct(request)
		if errValidate != nil {
			errorResponsesRequest := response.FormatErrorResp(errValidate, libValidate.GetTranslator())
			errorResponses = append(errorResponses, errorResponsesRequest...)
		}
	}
	if len(errorResponses) > 0 {
		return nil, nil, response.Error(http.StatusBadRequest, "VALIDATION_FAILED", errorResponses, libError.Join(err, "%s[ValidateRequest](fails)", name)), errorResponses
	}

	return &request, &req, nil, nil
}

func ParseRequest[Req any](
	w webFramework.WebFramework,
	mode Type,
	validateHeader bool,
) (*Req, *RequestHeader, response.Err) {
	var header RequestHeader

	const function = "ParseRequest"

	// bind the headers to data
	err := w.Parser.GetHeader(&header)
	if err != nil {
		return nil, nil, response.Error(http.StatusBadRequest, "HEADER_ABSENT", nil, libError.Join(err, "GetRequest[GetHeader](%v)", w.Parser.GetHttpHeader())).AddSource(function)
	}

	request, req, errParse, _ := parseRequest[Req](w, mode, validateHeader, &header, function)
	if errParse != nil {
		return nil, nil, errParse
	}

	w.Parser.SetLocal("reqLog", req)

	return request, &header, nil
}

func Req[Req any, Header any, PT interface {
	webFramework.HeaderInterface
	*Header
}](w webFramework.WebFramework, mode Type, validateHeader bool) (int, string, []response.ErrorResponse, Req, Request, error) {
	const function = "Req"

	// bind the headers to data
	header := new(Header)
	headerPtr := PT(header)
	errHeader := w.Parser.GetHeader(headerPtr)
	if errHeader != nil {
		return http.StatusBadRequest, "HEADER_ABSENT", nil, *new(Req), Request{}, libError.Join(errHeader, "%s[GetHeader](%v)", function, w.Parser.GetHttpHeader())
	}

	request, req, errParse, errArray := parseRequest[Req](w, mode, validateHeader, headerPtr, function)
	if errParse != nil {
		return errParse.Status, errParse.Description, errArray, *request, *req, *errParse
	}

	return http.StatusOK, "OK", nil, *request, *req, nil
}

func GetEmptyRequest(ctx webFramework.WebFramework) (int, string, []response.ErrorResponse, Request, error) {
	var req Request
	// bind the headers to data
	var header RequestHeader
	err := ctx.Parser.GetHeader(&header)
	if err != nil {
		return http.StatusBadRequest, "HEADER_ABSENT", nil, req, err
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
		errValidate := libValidate.ValidateStruct(header)
		if errValidate != nil {
			errorResponses := response.FormatErrorResp(errValidate, libValidate.GetTranslator())
			return http.StatusBadRequest, "Header Validation Failed", errorResponses, req, errValidate
		}
	}

	return http.StatusOK, "OK", nil, req, nil
}
