package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
)

type OrmHandlerType[Row, Resp any] struct {
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

func (q OrmHandlerType[Row, Resp]) Parameters() HandlerParameters {
	return HandlerParameters{q.Title, q.Mode, q.VerifyHeader, false, q.Path, false, q.RecoveryHandler, false, nil, nil}
}

func (q OrmHandlerType[Row, Resp]) Initializer(req HandlerRequest[Row, Resp]) error {
	return nil
}

func (q OrmHandlerType[Row, Resp]) CacheKey(args []any) string {
	return fmt.Sprintf("%s-%v", q.Title, args)
}

func (q OrmHandlerType[Row, Resp]) CheckCache(args []any) []Row {
	key := q.CacheKey(args)
	if data, ok := q.CacheData[key]; ok {
		if q.CacheTime.Add(q.CacheMaxAge).Before(time.Now()) {
			return data
		}
		delete(q.CacheData, key)
	}
	return nil
}

func (q OrmHandlerType[Row, Resp]) CacheResult(args []any, rows []Row) {
	key := q.CacheKey(args)
	q.CacheData[key] = rows
	q.CacheTime = time.Now()
}

func (q OrmHandlerType[Row, Resp]) Handler(req HandlerRequest[Row, Resp]) (Resp, error) {
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
		rows, err = liborm.GetQuery[Row](
			command,
			req.Core.ORM(),
			anyArgs...)
		if err != nil {
			if ok, errData := response.Unwrap(err); ok {
				if q.OnEmpty200 && errData.GetStatus() == http.StatusBadRequest &&
					errData.GetDescription() == libQuery.NO_DATA_FOUND {
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

func (q OrmHandlerType[Req, Resp]) Simulation(req HandlerRequest[Req, Resp]) (Resp, error) {
	return req.Response, nil
}

func (q OrmHandlerType[Req, Resp]) Finalizer(req HandlerRequest[Req, Resp]) {
}

func QueryWithOrm[Row, Resp any](
	core requestCore.RequestCoreInterface,
	handler OrmHandlerType[Row, Resp],
	simulation bool,
) any {
	return BaseHandler(core,
		handler,
		simulation)
}

func queryHandlerWithOrm[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	caching *CachingArgs,
	dbMode libQuery.DBMode,
) any {
	command := queryMap[key]
	var handler OrmHandlerType[Row, Resp]
	switch command.Type {
	case libQuery.QuerySingle:
		handler = OrmHandlerType[Row, Resp]{
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
		handler = OrmHandlerType[Row, Resp]{
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
	return QueryWithOrm(core, handler, simulation)
}

func QueryHandlerWithOrm[Row any, Resp []Row](
	title, key, path string, queryMap map[string]libQuery.QueryCommand,
	core requestCore.RequestCoreInterface,
	mode libRequest.Type,
	validateHeader, simulation bool,
	recoveryHandler func(any),
	caching *CachingArgs,
) any {
	return queryHandlerWithOrm[Row](
		title, key, path, queryMap,
		core, mode, validateHeader, simulation,
		recoveryHandler, caching, core.GetDB().GetDbMode(),
	)
}
