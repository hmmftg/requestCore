package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
)

// QueryResp holds the total row count and the translated response for a query.
type QueryResp[Resp any] struct {
	TotalRows int
	Resp      Resp
}

// RowTranslator is the interface for translating query rows into a response.
type RowTranslator[Row, Resp any] interface {
	Translate([]Row, HandlerRequest[Row, Resp]) (QueryResp[Resp], error)
	TranslateWithPaginate([]Row, HandlerRequest[Row, Resp], libRequest.PaginationData) (QueryResp[Resp], error)
}

// QuerySingleTransformer translates a single-row query result into a one-element slice response.
type QuerySingleTransformer[Row any, Resp []Row] struct {
}

// Translate wraps the first row into a single-element response.
func (s QuerySingleTransformer[Row, Resp]) Translate(rows []Row, _ HandlerRequest[Row, Resp]) (QueryResp[Resp], error) {
	return QueryResp[Resp]{
		TotalRows: 1,
		Resp:      Resp{rows[0]},
	}, nil
}

// TranslateWithPaginate wraps the first row into a single-element response with pagination.
func (s QuerySingleTransformer[Row, Resp]) TranslateWithPaginate(rows []Row, _ HandlerRequest[Row, Resp], _ libRequest.PaginationData) (QueryResp[Resp], error) {
	return QueryResp[Resp]{
		TotalRows: 1,
		Resp:      Resp{rows[0]},
	}, nil
}

// QueryAllTransformer translates all query rows into a slice response.
type QueryAllTransformer[Row any, Resp []Row] struct {
}

// Translate wraps all rows into a slice response.
func (s QueryAllTransformer[Row, Resp]) Translate(rows []Row, _ HandlerRequest[Row, Resp]) (QueryResp[Resp], error) {
	return QueryResp[Resp]{
		TotalRows: len(rows),
		Resp:      rows,
	}, nil
}

// TranslateWithPaginate wraps all rows into a slice response with pagination.
func (s QueryAllTransformer[Row, Resp]) TranslateWithPaginate(rows []Row, _ HandlerRequest[Row, Resp], _ libRequest.PaginationData) (QueryResp[Resp], error) {
	return QueryResp[Resp]{
		TotalRows: len(rows),
		Resp:      rows,
	}, nil
}

// QueryHandlerType holds configuration for a database query handler.
type QueryHandlerType[Row, Resp any] struct {
	Title           string
	Path            string
	Mode            libRequest.Type
	VerifyHeader    bool
	Key             string
	DbMode          libQuery.DBMode
	Command         libQuery.QueryCommand
	Translator      RowTranslator[Row, Resp]
	RecoveryHandler func(any)
	PaginateCommand func(string, libRequest.PaginationData) string
	Cache           bool
	CacheTime       time.Time
	CacheMaxAge     time.Duration
	CacheData       map[string][]Row
	OnEmpty200      bool
}

// CommandReplacer replaces a token in a query command string using a builder function.
type CommandReplacer[T any] struct {
	Token   string
	Builder func(T) string
}

// Replace substitutes the token in the command with the value built from data.
func (c CommandReplacer[T]) Replace(command string, data T) string {
	return strings.Replace(command, c.Token, c.Builder(data), 1)
}

// RowPaginator provides a less function for sorting rows based on pagination data.
type RowPaginator[Row any] struct {
	Less func(libRequest.PaginationData) func(i, j int) bool
}

const (
	// Asc is the ascending sort order constant.
	Asc = "asc"
	// Dsc is the descending sort order constant.
	Dsc = "desc"
)

// Filter holds a single filter condition for query result filtering.
type Filter struct {
	Field    string
	Operator string
	Value    string
	Value2nd string
}

// Filterate applies filter conditions from pagination data to the row slice.
func Filterate[Row any](paginationData libRequest.PaginationData, data []Row, filterFunc func(Filter) func(Row) bool) []Row {
	if len(paginationData.Filters) == 0 {
		return data
	}
	filterList := strings.Split(paginationData.Filters, " and ")
	if len(filterList) <= 0 {
		return data
	}
	result := slices.Clone(data)
	for id := range filterList {
		split := strings.Split(filterList[id], " ")
		if len(split) != 4 {
			for i := len(split); i < 4; i++ {
				split = append(split, " ")
			}
		}
		result = slices.DeleteFunc(
			result,
			filterFunc(
				Filter{
					Field:    split[0],
					Operator: split[1],
					Value:    split[2],
					Value2nd: split[3],
				},
			))
	}

	return result
}

// Paginate sorts and slices the data according to the pagination parameters.
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

// Parameters returns the handler parameters for the QueryHandlerType.
func (q QueryHandlerType[Row, Resp]) Parameters() HandlerParameters[Row, Resp] {
	return HandlerParameters[Row, Resp]{
		Title:           q.Title,
		Body:            q.Mode,
		ValidateHeader:  q.VerifyHeader,
		Persistence:     nil,
		Path:            q.Path,
		HasReceipt:      false,
		RecoveryHandler: q.RecoveryHandler,
		FileResponse:    false,
		LogArrays:       nil,
		LogTags:         nil,
		EnableTracing:   false,
		TracingSpanName: "",
	}
}

// Initializer is a no-op initializer for the QueryHandlerType.
func (q QueryHandlerType[Row, Resp]) Initializer(_ HandlerRequest[Row, Resp]) error {
	return nil
}

// CacheKey builds a cache key from the handler title and arguments.
func (q QueryHandlerType[Row, Resp]) CacheKey(args []any) string {
	return fmt.Sprintf("%s-%v", q.Title, args)
}

// CheckCache returns cached rows for the given arguments if still valid.
func (q QueryHandlerType[Row, Resp]) CheckCache(args []any) []Row {
	key := q.CacheKey(args)
	if data, ok := q.CacheData[key]; ok {
		if q.CacheTime.Add(q.CacheMaxAge).Before(time.Now()) {
			return data
		}
		delete(q.CacheData, key)
	}
	return nil
}

// CacheResult stores rows in the cache under the key derived from the arguments.
func (q QueryHandlerType[Row, Resp]) CacheResult(args []any, rows []Row) {
	key := q.CacheKey(args)
	q.CacheData[key] = rows
	q.CacheTime = time.Now()
}

// Handler executes the query and translates the rows into the response.
func (q QueryHandlerType[Row, Resp]) Handler(req HandlerRequest[Row, Resp]) (Resp, error) {
	anyArgs := []any{}
	for id := range q.Command.Args {
		_, val, err := libQuery.GetFormTagValue(q.Command.Args[id].(string), req.Request)
		if err != nil {
			return req.Response, errors.Join(err, libError.NewWithDescription(
				status.InternalServerError,
				"COMMAND_ARGUMENT_ERROR",
				"command argument eror: %s", q.Command,
			))
		}
		anyArgs = append(anyArgs, *val)
	}
	var rows []Row
	var err error
	if q.Cache {
		rows = q.CheckCache(anyArgs)
	}
	if rows == nil {
		command := q.Command.Command
		if len(q.Command.CommandMap) > 0 && len(q.Command.CommandMap[q.DbMode]) > 0 {
			command = q.Command.CommandMap[q.DbMode]
		}

		if q.PaginateCommand != nil {
			if q.Mode == libRequest.QueryWithPagination || q.Mode == libRequest.URIAndPagination {
				pgData, ok := req.W.Parser.GetLocal(libRequest.PaginationLocalTag).(libRequest.PaginationData)
				if ok {
					command = q.PaginateCommand(command, pgData)
				}
			}
		}
		rows, err = libQuery.GetQuery[Row](
			command,
			req.Core.GetDB(),
			anyArgs...)
		if err != nil {
			if ok, errData := response.Unwrap(err); ok {
				if q.OnEmpty200 && errData.GetStatus() == http.StatusBadRequest &&
					errData.GetDescription() == libQuery.NoDataFound {
					rows = []Row{}
				}
			} else {
				return req.Response, err
			}
		}

		if q.Cache {
			q.CacheResult(anyArgs, rows)
		}
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

	req.W.Parser.SetRespHeader("X-Total-Count", fmt.Sprintf("%d", resp.TotalRows))

	return resp.Resp, nil

}

// Simulation returns the default response for simulation mode.
func (q QueryHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	return req.Response, nil
}

// Finalizer is a no-op finalizer for the QueryHandlerType.
func (q QueryHandlerType[Req, Resp]) Finalizer(_ HandlerRequest[Req, Resp]) {
}

// Query returns a base handler that executes a database query.
func Query[Row, Resp any](
	core requestCore.RequestCoreInterface,
	handler QueryHandlerType[Row, Resp],
	simulation bool,
) any {
	return BaseHandler(core,
		handler,
		simulation)
}

// CachingArgs holds caching configuration for query handlers.
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
	dbMode libQuery.DBMode,
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
			DbMode:          dbMode,
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
			DbMode:          dbMode,
		}
	default:
		log.Fatalln("invalid command type", command.Type)
		return nil
	}
	if caching != nil {
		handler.Cache = caching.Cache
		handler.CacheMaxAge = caching.CacheMaxAge
		handler.CacheData = map[string][]Row{}
	}
	return Query(core, handler, simulation)
}

// QueryHandler returns a base handler for database queries using the database's default mode.
func QueryHandler[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
) any {
	return queryHandler[Row](title, key, path, queryMap,
		core,
		mode,
		validateHeader, simulation,
		recoveryHandler, nil, core.GetDB().GetDbMode())
}

// QueryHandlerWithCaching returns a base handler for database queries with caching support.
func QueryHandlerWithCaching[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	caching *CachingArgs,
) any {
	return queryHandler[Row](
		title, key, path, queryMap,
		core, mode, validateHeader, simulation,
		recoveryHandler, caching, core.GetDB().GetDbMode(),
	)
}

// QueryHandlerWithTransform returns a base handler for queries with a custom translator and command replacer.
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
		handler.CacheData = map[string][]Row{}
	}
	return Query(core,
		handler,
		simulation)
}
