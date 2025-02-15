package requestCore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type Empty struct {
}

func GetSingleRecordHandler[Req, Resp any](title, sql string,
	core RequestCoreInterface,
) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		id := w.Parser.GetUrlParam("id")
		id = strings.ReplaceAll(id, "*", "/")
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}
		code, desc, data, respRaw, err := libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, data, w)
			return
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, w)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			core.Responder().HandleErrorState(
				libError.Join(err, "%s[json.Unmarsha](%s)", title, respRaw[0].DataRaw),
				http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, w)
			return
		}
		core.Responder().Respond(200, 0, "OK", result[0], false, w)
	}
}

func GetMapHandler[Req any, Resp webFramework.RecordData](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, w)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			core.Responder().HandleErrorState(
				libError.Join(err, "%s[json.Unmarsha](%s)", title, respRaw[0].DataRaw),
				http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, w)
			return
		}
		respMap := make(map[string]any, 0)
		for _, row := range result {
			respMap[row.GetId()] = row.GetValue()
		}
		core.Responder().Respond(200, 0, "OK", respMap, false, w)
	}
}

func GetMapBySubHandler[Req any, Resp webFramework.RecordData](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, w)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			core.Responder().HandleErrorState(
				libError.Join(err, "%s[json.Unmarsha](%s)", title, respRaw[0].DataRaw),
				http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, w)
			return
		}
		respMap := make(map[string]map[string]any, 0)
		for _, row := range result {
			if _, ok := respMap[row.GetSubCategory()]; ok {
				respMap[row.GetSubCategory()][row.GetId()] = row.GetValue()
			} else {
				respMap[row.GetSubCategory()] = make(map[string]any, 0)
				respMap[row.GetSubCategory()][row.GetId()] = row.GetValue()
			}
		}
		core.Responder().Respond(200, 0, "OK", respMap, false, w)
	}
}

func GetQuery[Req any](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, w)
			return
		}
		var result []map[string]any
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			core.Responder().HandleErrorState(
				libError.Join(err, "%s[json.Unmarsha](%s)", title, respRaw[0].DataRaw),
				http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, w)
			return
		}
		core.Responder().Respond(200, 0, "OK", result, false, w)
	}
}

func GetQueryMap[Req any](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, w)
				return
			}
		}

		if len(respRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, w)
			return
		}
		result := make([]any, 0)
		for _, row := range respRaw {
			if len(row.MapList) > 0 {
				var mapData []map[string]map[string]any
				err = json.Unmarshal([]byte(row.MapList), &mapData)
				if err != nil {
					core.Responder().HandleErrorState(
						libError.Join(err, "%s[json.Unmarsha](%s)", title, row.MapList),
						http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", row.MapList, w)
					return
				}
				mapFlat := make([]any, 0)
				for _, subData := range mapData {
					subMapFlat := make(map[string]any, 0)
					for id, data := range subData {
						subMapFlat[id] = data
					}
					mapFlat = append(mapFlat, subMapFlat)
				}
				mapResult := make(map[string][]any, 0)
				mapResult[row.Key] = mapFlat
				result = append(result, mapResult)
				//result[row.Key] = mapFlat
			}
		}
		core.Responder().Respond(200, 0, "OK", result, false, w)
	}
}

func GetQueryHandler[Req, Resp any](title, sql string,
	core RequestCoreInterface,
	args ...any) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().Respond(code, 1, desc, arrayErr, true, w)
			return
		}
		code, desc, data, respRaw, err := libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
		if err != nil {
			core.Responder().Respond(code, 1, desc, data, true, w)
			return
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, w)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			core.Responder().HandleErrorState(
				libError.Join(err, "%s[json.Unmarsha](%s)", title, respRaw[0].DataRaw),
				http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, w)
			return
		}
		core.Responder().Respond(200, 0, "OK", result, false, w)
	}
}

func GetQueryFillable[Resp libQuery.QueryWithDeps](
	title, query string,
	core RequestCoreInterface,
	args ...string,
) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		params := []any{}
		for _, arg := range args {
			val, exists := w.Parser.CheckUrlParam(arg)
			if exists {
				params = append(params, val)
			}
		}
		code, desc, data, result, err := libQuery.CallSql[Resp](query, core.GetDB(), params...)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, data, w)
			return
		}
		if len(result) == 0 {
			core.Responder().Respond(http.StatusBadRequest, 1, libQuery.NO_DATA_FOUND, "No Data Found", true, w)
			return
		}
		var filledResp []Resp
		for _, row := range result {
			fillResp, err := row.GetFillable(core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(
					libError.Join(err, "%s[GetFillable](%v)=>%v", title, row, fillResp),
					code, fillResp["Desc"].(string), fillResp["Data"], w)
				return
			}
			filledResp = append(filledResp, fillResp["Filled"].(Resp))
		}
		core.Responder().Respond(http.StatusOK, 0, "OK", filledResp, false, w)
	}
}

type MapHandler interface {
	GetAllMap(
		core RequestCoreInterface, args ...any,
	) (map[string]any, string, error)
}

func GetAllMapHandler[Model MapHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		var model Model
		result, desc, err := model.GetAllMap(core)
		if err != nil {
			respHandler.Respond(http.StatusBadRequest, 1, desc, err.Error(), true, w)
			return
		}
		if len(result) == 0 {
			respHandler.Respond(http.StatusBadRequest, 1, libQuery.NO_DATA_FOUND, "No Data Found", true, w)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, w)
	}
}

type GetHandler interface {
	GetSingle(
		core RequestCoreInterface, args map[string]string,
	) (any, string, error)
	GetAll(
		core RequestCoreInterface, args map[string]string,
	) (any, string, error)
}

func GetParams(w webFramework.WebFramework, args ...any) map[string]string {
	params := make(map[string]string, 0)
	for _, arg := range args {
		val, exists := w.Parser.CheckUrlParam(arg.(string))
		if exists {
			params[arg.(string)] = val
		} else {
			params[arg.(string)] = arg.(string)
		}
	}
	return params
}

func GetSingleRecord[Model GetHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		params := GetParams(w, args...)
		code, desc, arrayErr, model, _, err := libRequest.GetRequest[Model](w, false)
		if err != nil {
			respHandler.Respond(code, 1, desc, arrayErr, true, w)
			return
		}
		result, desc, err := model.GetSingle(core, params)
		if err != nil {
			respHandler.Respond(http.StatusBadRequest, 1, desc, err.Error(), true, w)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, w)
	}
}

func GetAll[Model GetHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		params := GetParams(w, args...)
		code, desc, arrayErr, model, _, err := libRequest.GetRequest[Model](w, false)
		if err != nil {
			respHandler.Respond(code, 1, desc, arrayErr, true, w)
			return
		}
		result, desc, err := model.GetAll(core, params)
		if err != nil {
			respHandler.Respond(http.StatusBadRequest, 1, desc, err.Error(), true, w)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, w)
	}
}

type GetPageHandler interface {
	GetPage(
		core RequestCoreInterface,
		pageSize, pageId int,
		args map[string]string,
	) (any, string, error)
	GetPageParams() (int, int)
}

func GetPage[Model GetPageHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		params := GetParams(w, args...)
		code, desc, arrayErr, model, _, err := libRequest.GetRequest[Model](w, false)
		if err != nil {
			respHandler.Respond(code, 1, desc, arrayErr, true, w)
			return
		}
		pageSize, pageId := model.GetPageParams()
		result, desc, err := model.GetPage(core, pageSize, pageId, params)
		if err != nil {
			respHandler.HandleErrorState(err, http.StatusBadRequest, desc, err.Error(), w)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, w)
	}
}

func QueryHandler[Req libQuery.QueryRequest, Resp libQuery.QueryResult](
	title, key string, queryMap map[string]libQuery.QueryCommand,
	core RequestCoreInterface,
	mode libRequest.Type,
	validateHeader bool,
) any {
	webFramework.AddServiceRegistrationLog(title)
	return func(c context.Context) {
		defer func() {
			w := libContext.InitContextNoAuditTrail(c)
			if r := recover(); r != nil {
				core.Responder().HandleErrorState(libError.Join(r.(error), "error in query"), http.StatusInternalServerError, response.SYSTEM_FAULT, response.SYSTEM_FAULT_DESC, w)
				panic(r)
			}
		}()
		w := libContext.InitContextNoAuditTrail(c)
		code, desc, arrayErr, request, _, err := libRequest.Req[Req, libRequest.RequestHeader](w, mode, validateHeader)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
			return
		}

		core.RequestTools().LogStart(w, fmt.Sprintf("Query: %s", queryMap[key].Name), "Call")
		args := make([]any, 0)
		if !reflect.ValueOf(&request).Elem().IsZero() {
			args = request.QueryArgs()[key]
		}
		resp, errQuery := libQuery.Query[Resp](core.GetDB(), queryMap[key], args...)
		if errQuery != nil {
			core.Responder().HandleErrorState(libError.Join(errQuery, "query"), http.StatusBadRequest, errQuery.GetDescription(), errQuery.GetMessage(), w)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, w)
	}
}
