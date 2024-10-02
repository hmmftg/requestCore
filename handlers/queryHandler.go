package handlers

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type QueryResp[Resp any] struct {
	TotalRows int
	Resp      Resp
}

type RowTranslator[Row, Resp any] interface {
	Translate([]Row, HandlerRequest[Row, Resp]) (QueryResp[Resp], response.ErrorState)
	TranslateWithPaginate([]Row, HandlerRequest[Row, Resp], libRequest.PaginationData) (QueryResp[Resp], response.ErrorState)
}

type QuerySingleTransformer[Row any, Resp []Row] struct {
}

func (s QuerySingleTransformer[Row, Resp]) Translate(rows []Row, req HandlerRequest[Row, Resp]) (QueryResp[Resp], response.ErrorState) {
	return QueryResp[Resp]{
		TotalRows: 1,
		Resp:      Resp{rows[0]},
	}, nil
}

func (s QuerySingleTransformer[Row, Resp]) TranslateWithPaginate(rows []Row, req HandlerRequest[Row, Resp], pd libRequest.PaginationData) (QueryResp[Resp], response.ErrorState) {
	return QueryResp[Resp]{
		TotalRows: 1,
		Resp:      Resp{rows[0]},
	}, nil
}

type QueryAllTransformer[Row any, Resp []Row] struct {
}

func (s QueryAllTransformer[Row, Resp]) Translate(rows []Row, req HandlerRequest[Row, Resp]) (QueryResp[Resp], response.ErrorState) {
	return QueryResp[Resp]{
		TotalRows: len(rows),
		Resp:      rows,
	}, nil
}

func (s QueryAllTransformer[Row, Resp]) TranslateWithPaginate(rows []Row, req HandlerRequest[Row, Resp], pd libRequest.PaginationData) (QueryResp[Resp], response.ErrorState) {
	return QueryResp[Resp]{
		TotalRows: len(rows),
		Resp:      rows,
	}, nil
}

type QueryHandlerType[Row, Resp any] struct {
	Title           string
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	Key             string
	Command         libQuery.QueryCommand
	Translator      RowTranslator[Row, Resp]
	RecoveryHandler func(any)
	PaginateCommand func(string, libRequest.PaginationData) string
	Cache           bool
	CacheTime       time.Time
	CacheMaxAge     time.Duration
	CacheData       map[string]Resp
}

type CommandReplacer[T any] struct {
	Token   string
	Builder func(T) string
}

func (c CommandReplacer[T]) Replace(command string, data T) string {
	return strings.Replace(command, c.Token, c.Builder(data), 1)
}

type RowPaginator[Row any] struct {
	Less func(libRequest.PaginationData) func(i, j int) bool
}

const (
	Asc = "asc"
	Dsc = "desc"
)

type Filter struct {
	Field    string
	Operator string
	Value    string
}

func Filterate[Row any](paginationData libRequest.PaginationData, data []Row, filterFunc func(Filter) func(Row) bool) []Row {
	if len(paginationData.Filters) == 0 {
		return data
	}
	filterList := strings.Split(paginationData.Filters, " and ")
	if len(filterList) <= 0 {
		return data
	}
	result := data
	for id := range filterList {
		split := strings.Split(filterList[id], " ")
		result = slices.DeleteFunc(
			result,
			filterFunc(
				Filter{
					Field:    split[0],
					Operator: split[1],
					Value:    split[2],
				},
			))
	}

	return result
}

func Paginate[Row any](paginationData libRequest.PaginationData, data []Row, less func(string) func(i int, j int) bool) []Row {
	start := paginationData.Start
	end := paginationData.End
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end == start && start == 0 && len(data) > 1 {
		end = len(data)
	}
	if end > len(data) {
		end = len(data)
	}
	result := data
	if len(paginationData.Sort) > 0 {
		sort.Slice(result, less(paginationData.Sort))
	}
	if paginationData.Order == Dsc {
		slices.Reverse(result)
	}
	return result[start:end]
}

func (q QueryHandlerType[Row, Resp]) Parameters() HandlerParameters {
	return HandlerParameters{q.Title, q.Mode, q.VerifyHeader, false, q.Path, false, q.RecoveryHandler, false}
}

func (q QueryHandlerType[Row, Resp]) Initializer(req HandlerRequest[Row, Resp]) response.ErrorState {
	return nil
}

func (q QueryHandlerType[Row, Resp]) CacheKey(args []any) string {
	return fmt.Sprintf("%s-%v", q.Title, args)
}

type Cacheable struct {
	SetTime time.Time
	Data    any
}

var CacheData = map[string]Cacheable{}

func (q QueryHandlerType[Row, Resp]) CheckCache(args []any) *Resp {
	key := q.CacheKey(args)
	if data, ok := q.CacheData[key]; ok {
		if q.CacheTime.Add(q.CacheMaxAge).Before(time.Now()) {
			return &data
		}
		delete(q.CacheData, key)
	}
	return nil
}

func (q QueryHandlerType[Row, Resp]) CacheResult(args []any, resp Resp) {
	key := q.CacheKey(args)
	q.CacheData[key] = resp
	q.CacheTime = time.Now()
}

func (q QueryHandlerType[Row, Resp]) Handler(req HandlerRequest[Row, Resp]) (Resp, response.ErrorState) {
	anyArgs := []any{}
	for id := range q.Command.Args {
		_, val, err := libQuery.GetFormTagValue(q.Command.Args[id], req.Request)
		if err != nil {
			return req.Response, response.Error(
				http.StatusInternalServerError,
				"COMMAND_ARGUMENT_ERROR",
				q.Command,
				err)
		}
		anyArgs = append(anyArgs, *val)
	}
	if q.Cache {
		data := q.CheckCache(anyArgs)
		if data != nil {
			return *data, nil
		}
	}

	command := q.Command.Command
	if q.PaginateCommand != nil {
		if q.Mode == libRequest.QueryWithPagination || q.Mode == libRequest.URIAndPagination {
			pgData, ok := req.W.Parser.GetLocal(libRequest.PaginationLocalTag).(libRequest.PaginationData)
			if ok {
				command = q.PaginateCommand(command, pgData)
			}
		}
	}
	rows, err := libQuery.GetQuery[Row](
		command,
		req.Core.GetDB(),
		anyArgs...)
	if err != nil {
		return req.Response, err
	}
	paginate := false
	var pgData libRequest.PaginationData
	if q.Mode == libRequest.QueryWithPagination || q.Mode == libRequest.URIAndPagination {
		pgData, paginate = req.W.Parser.GetLocal(libRequest.PaginationLocalTag).(libRequest.PaginationData)
	}
	var resp QueryResp[Resp]
	if paginate {
		resp, err = q.Translator.TranslateWithPaginate(rows, req, pgData)
	} else {
		resp, err = q.Translator.Translate(rows, req)
	}
	if err != nil {
		return req.Response, err
	}

	if q.Cache {
		q.CacheResult(anyArgs, resp.Resp)
	}

	req.W.Parser.SetRespHeader("X-Total-Count", fmt.Sprintf("%d", resp.TotalRows))

	return resp.Resp, nil

}

func (q QueryHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState) {
	return req.Response, nil
}

func (q QueryHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {
}

type CachingArgs struct {
	Cache       bool
	CacheMaxAge time.Duration
}

func queryHandler[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	caching *CachingArgs,
) any {
	command := queryMap[key]
	var handler QueryHandlerType[Row, Resp]
	switch command.Type {
	case libQuery.QuerySingle:
		handler = QueryHandlerType[Row, Resp]{
			Mode:            mode,
			VerifyHeader:    validateHeader,
			Title:           title,
			Key:             key,
			Command:         command,
			Path:            path,
			Translator:      QuerySingleTransformer[Row, Resp]{},
			RecoveryHandler: recoveryHandler,
		}
	case libQuery.QueryAll:
		handler = QueryHandlerType[Row, Resp]{
			Mode:            mode,
			VerifyHeader:    validateHeader,
			Title:           title,
			Key:             key,
			Command:         command,
			Path:            path,
			Translator:      QueryAllTransformer[Row, Resp]{},
			RecoveryHandler: recoveryHandler,
		}
	default:
		log.Fatalln("invalid command type", command.Type)
		return nil
	}
	if caching != nil {
		handler.Cache = caching.Cache
		handler.CacheMaxAge = caching.CacheMaxAge
		handler.CacheData = map[string]Resp{}
	}
	return BaseHandler(core, handler, simulation)
}

func QueryHandler[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
) any {
	return queryHandler[Row, Resp](title, key, path, queryMap,
		core,
		mode,
		validateHeader, simulation,
		recoveryHandler, nil)
}

func QueryHandlerWithCaching[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	caching *CachingArgs,
) any {
	return queryHandler[Row, Resp](
		title, key, path, queryMap,
		core, mode, validateHeader, simulation,
		recoveryHandler, caching,
	)
}

func QueryHandlerWithTransform[Row, Resp any](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	replacer CommandReplacer[libRequest.PaginationData],
	translator RowTranslator[Row, Resp],
	caching *CachingArgs,
) any {
	command := queryMap[key]
	command.Type = libQuery.Transforms
	handler := QueryHandlerType[Row, Resp]{
		Mode:            mode,
		VerifyHeader:    validateHeader,
		Title:           title,
		Key:             key,
		Command:         command,
		Path:            path,
		Translator:      translator,
		RecoveryHandler: recoveryHandler,
		PaginateCommand: replacer.Replace,
	}
	if caching != nil {
		handler.Cache = caching.Cache
		handler.CacheMaxAge = caching.CacheMaxAge
		handler.CacheData = map[string]Resp{}
	}
	return BaseHandler(core,
		handler,
		simulation)
}
