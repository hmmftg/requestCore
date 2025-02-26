package libRequest

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libValidate"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

func GetRequest[Q any](w webFramework.WebFramework, isJson bool) (*ParseResult[Q], error) {
	params := ParseParams{
		W:              w,
		ValidateHeader: w.Parser.GetMethod() != "GET",
	}
	if isJson {
		params.Mode = JSON
	} else {
		params.Mode = Query
	}
	return Req[Q, RequestHeader](params)
}

//go:generate enumer -type=Type -json -output requestTypeEnum.go
type Type int

const (
	NoBinding Type = iota
	JSON
	JSONWithURI
	Query
	QueryWithURI
	QueryWithPagination
	URI
	URIAndPagination
)

const PaginationLocalTag string = "pagination"

type PaginationData struct {
	Start   int    `form:"_start" query:"_start" validate:"omitempty"`
	End     int    `form:"_end" query:"_end" validate:"omitempty"`
	Filters string `form:"_filters" query:"_filters" validate:"omitempty"`
	Sort    string `form:"_sort" query:"_sort" validate:"omitempty"`
	Order   string `form:"_order" query:"_order" validate:"omitempty,oneof=asc desc"`
}

const ErrorInGetRequest = "ERROR_IN_GET_REQUEST_%s"

type ParseParams struct {
	W              webFramework.WebFramework
	Mode           Type
	ValidateHeader bool
	Header         webFramework.HeaderInterface
	Name           string
	StoreTags      []string
	StoreHeaders   []string
}

func parseRequest[Req any](params ParseParams) (*ParseResult[Req], error) {
	libValidate.Init()
	var err error
	var request Req

	//Check Input
	var desc string
	switch params.Mode {
	case JSON:
		err = params.W.Parser.GetBody(&request)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "BODY")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetBody](fails)", params.Name)
		}
	case JSONWithURI:
		err = params.W.Parser.GetBody(&request)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "BODY")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetBody](fails)", params.Name)
		}
		errUri := params.W.Parser.GetUri(&request)
		if errUri != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "URI")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUri](fails)", params.Name)
		}
	case Query:
		err = params.W.Parser.GetUrlQuery(&request)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "QUERY")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUrlQuery](fails)", params.Name)
		}
	case QueryWithPagination:
		var pagination PaginationData
		err = params.W.Parser.GetUrlQuery(&pagination)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "PAGINATION")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetPaginationQuery](fails)", params.Name)
		} else {
			params.W.Parser.SetLocal(PaginationLocalTag, pagination)
		}
		errQuery := params.W.Parser.GetUrlQuery(&request)
		if errQuery != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "QUERY")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUrlQuery](fails)", params.Name)
		}
	case QueryWithURI:
		err = params.W.Parser.GetUrlQuery(&request)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "QUERY")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUrlQuery](fails)", params.Name)
		}
		errUri := params.W.Parser.GetUri(&request)
		if errUri != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "URI")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUri](fails)", params.Name)
		}
	case URI:
		err = params.W.Parser.GetUri(&request)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "URI")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUri](fails)", params.Name)
		}
	case URIAndPagination:
		var pagination PaginationData
		err = params.W.Parser.GetUrlQuery(&pagination)
		if err != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "PAGINATION")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetPaginationQuery](fails)", params.Name)
		} else {
			params.W.Parser.SetLocal(PaginationLocalTag, pagination)
		}
		errUri := params.W.Parser.GetUri(&request)
		if errUri != nil {
			desc = fmt.Sprintf(ErrorInGetRequest, "URI")
			err = libError.NewWithDescription(status.BadRequest, desc, "%s[GetUri](fails)", params.Name)
		}
	default:
		err = nil
	}
	if err != nil {
		return nil, err
	}

	req := Request{
		Header:   params.Header,
		Id:       params.Header.GetId(),
		Time:     time.Now(),
		Incoming: request, //string(requestJson),
	}
	req.Tags = map[string]string{}
	for id := range params.StoreTags {
		req.Tags[params.StoreTags[id]] = params.W.Parser.GetLocalString(params.StoreTags[id])
	}
	for id := range params.StoreHeaders {
		req.Tags[params.StoreHeaders[id]] = params.W.Parser.GetHeaderValue(params.StoreHeaders[id])
	}

	errorResponses := []response.ErrorResponse{}
	if params.ValidateHeader {
		err, errValidate := libValidate.ValidateStruct(params.Header)
		if err != nil {
			return nil, errors.Join(libError.NewWithDescription(status.InternalServerError, "INVALID_HEADER_VALIDATION", "invalid header validation for %T", params.Header), err)
		}
		if errValidate != nil {
			errorResponsesHeader := response.FormatErrorResp(errValidate, libValidate.GetTranslator())
			errorResponses = append(errorResponses, errorResponsesHeader...)
		}
	}

	if params.Mode != NoBinding {
		err, errValidate := libValidate.ValidateStruct(request)
		if err != nil {
			return nil, errors.Join(libError.NewWithDescription(status.InternalServerError, "INVALID_VALIDATION", "invalid body validation for %T", request), err)
		}
		if errValidate != nil {
			errorResponsesRequest := response.FormatErrorResp(errValidate, libValidate.GetTranslator())
			errorResponses = append(errorResponses, errorResponsesRequest...)
		}
	}
	if len(errorResponses) > 0 {
		return nil, errors.Join(libError.New(http.StatusBadRequest, "VALIDATION_FAILED", errorResponses))
	}

	return &ParseResult[Req]{request, &req}, nil
}

func ParseRequest[Req any](
	w webFramework.WebFramework,
	mode Type,
	validateHeader bool,
) (*Req, *RequestHeader, error) {
	var header RequestHeader

	const function = "ParseRequest"

	// bind the headers to data
	err := w.Parser.GetHeader(&header)
	if err != nil {
		return nil, nil, errors.Join(err,
			libError.NewWithDescription(
				status.BadRequest,
				"HEADER_ABSENT",
				"header absent in %s.GetRequest[GetHeader](%v)", function, w.Parser.GetHttpHeader()))
	}

	params := ParseParams{
		W:              w,
		Mode:           mode,
		ValidateHeader: validateHeader,
		Header:         &header,
		Name:           function,
	}
	result, err := parseRequest[Req](params)
	if err != nil {
		return nil, nil, err
	}

	result.RequestPtr.Incoming = result.Request

	w.Parser.SetLocal("reqLog", result.RequestPtr)

	return &result.Request, &header, nil
}

type ParseResult[Req any] struct {
	Request    Req
	RequestPtr RequestPtr
}

func Req[Req any, Header any, PT interface {
	webFramework.HeaderInterface
	*Header
}](params ParseParams) (*ParseResult[Req], error) {
	const function = "Req"

	// bind the headers to data
	header := new(Header)
	headerPtr := PT(header)
	err := params.W.Parser.GetHeader(headerPtr)
	if err != nil {
		return nil, errors.Join(err, libError.NewWithDescription(http.StatusBadRequest, "HEADER_ABSENT", "%s[GetHeader](%v)", function, params.W.Parser.GetHttpHeader()))
	}

	params.Header = headerPtr

	result, err := parseRequest[Req](params)
	if err != nil {
		return nil, err
	}

	return result, nil
}
