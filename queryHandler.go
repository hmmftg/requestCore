package requestCore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type Empty struct {
}

func GetSingleRecordHandler[Req, Resp any](title, sql string,
	core RequestCoreInterface,
) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		id := w.Parser.GetUrlParam("id")
		id = strings.ReplaceAll(id, "*", "/")
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		code, desc, data, respRaw, err := libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, data, c)
			return
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf("NO_DATA_FOUND"), http.StatusBadRequest, "NO_DATA_FOUND", arrayErr, c)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			fmt.Println(title, "Unmarshal DataRaw", err, respRaw[0].DataRaw)
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, c)
			return
		}
		core.Responder().Respond(200, 0, "OK", result[0], false, c)
	}
}

func GetMapHandler[Req any, Resp libQuery.RecordData](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf("NO_DATA_FOUND"), http.StatusBadRequest, "NO_DATA_FOUND", arrayErr, c)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			fmt.Println(title, "Unmarshal DataRaw", err, respRaw[0].DataRaw)
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, c)
			return
		}
		respMap := make(map[string]any, 0)
		for _, row := range result {
			respMap[row.GetId()] = row.GetValue()
		}
		core.Responder().Respond(200, 0, "OK", respMap, false, c)
	}
}

func GetMapBySubHandler[Req any, Resp libQuery.RecordData](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf("NO_DATA_FOUND"), http.StatusBadRequest, "NO_DATA_FOUND", arrayErr, c)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			fmt.Println(title, "Unmarshal DataRaw", err, respRaw[0].DataRaw)
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, c)
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
		core.Responder().Respond(200, 0, "OK", respMap, false, c)
	}
}

func GetQuery[Req any](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf("NO_DATA_FOUND"), http.StatusBadRequest, "NO_DATA_FOUND", arrayErr, c)
			return
		}
		var result []map[string]any
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			fmt.Println(title, "Unmarshal DataRaw", err, respRaw[0].DataRaw)
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, c)
			return
		}
		core.Responder().Respond(200, 0, "OK", result, false, c)
	}
}

func GetQueryMap[Req any](title, sql string,
	core RequestCoreInterface,
	hasParam bool) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		id := ""
		if hasParam {
			id = w.Parser.GetUrlParam("id")
			id = strings.ReplaceAll(id, "*", "/")
		}
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		var respRaw []libQuery.QueryData
		var data any
		if len(id) == 0 {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		} else {
			code, desc, data, respRaw, err = libQuery.CallSql[libQuery.QueryData](sql, core.GetDB(), id)
			if err != nil {
				core.Responder().HandleErrorState(err, code, desc, data, c)
				return
			}
		}

		if len(respRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf("NO_DATA_FOUND"), http.StatusBadRequest, "NO_DATA_FOUND", arrayErr, c)
			return
		}
		result := make([]any, 0)
		for _, row := range respRaw {
			if len(row.MapList) > 0 {
				var mapData []map[string]map[string]any
				err = json.Unmarshal([]byte(row.MapList), &mapData)
				if err != nil {
					fmt.Println(title, "Unmarshal DataRaw", err, row.MapList)
					core.Responder().HandleErrorState(err, http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", row.MapList, c)
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
		core.Responder().Respond(200, 0, "OK", result, false, c)
	}
}

func GetQueryHandler[Req, Resp any](title, sql string,
	core RequestCoreInterface,
	args ...any) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		code, desc, arrayErr, _, _, err := libRequest.GetRequest[Req](w, false)
		if err != nil {
			core.Responder().Respond(code, 1, desc, arrayErr, true, c)
			return
		}
		code, desc, data, respRaw, err := libQuery.CallSql[libQuery.QueryData](sql, core.GetDB())
		if err != nil {
			core.Responder().Respond(code, 1, desc, data, true, c)
			return
		}

		if len(respRaw[0].DataRaw) == 0 {
			core.Responder().HandleErrorState(fmt.Errorf("NO_DATA_FOUND"), http.StatusBadRequest, "NO_DATA_FOUND", arrayErr, c)
			return
		}

		var result []Resp
		err = json.Unmarshal([]byte(respRaw[0].DataRaw), &result)
		if err != nil {
			fmt.Println(title, "Unmarshal DataRaw", err, respRaw[0].DataRaw)
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, "DB_RESP_PARSE_ERROR", respRaw[0].DataRaw, c)
			return
		}
		core.Responder().Respond(200, 0, "OK", result, false, c)
	}
}

func GetQueryFillable[Resp libQuery.QueryWithDeps](
	title, query string,
	core RequestCoreInterface,
	args ...string,
) any {
	fmt.Println(title + "...")
	return func(c any) {
		w := libContext.InitContext(c)
		params := []any{}
		for _, arg := range args {
			val, exists := w.Parser.CheckUrlParam(arg)
			if exists {
				params = append(params, val)
			}
		}
		code, desc, data, result, err := libQuery.CallSql[Resp](query, core.GetDB(), params...)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, data, c)
			return
		}
		if len(result) == 0 {
			core.Responder().Respond(http.StatusBadRequest, 1, "PWC_WS_0008", "No Data Found", true, c)
			return
		}
		var filledResp []Resp
		for _, row := range result {
			fillResp, err := row.GetFillables(core.GetDB())
			if err != nil {
				fmt.Println(fillResp["Desc"], fillResp["Field"], fillResp["Data"])
				core.Responder().Respond(code, 1, fillResp["Desc"].(string), fillResp["Data"], true, c)
				return
			}
			filledResp = append(filledResp, fillResp["Filled"].(Resp))
		}
		core.Responder().Respond(http.StatusOK, 0, "OK", filledResp, false, c)
	}
}

type MapHandler interface {
	GetAllMap(
		core RequestCoreInterface,
	) (map[string]any, string, error)
}

func GetAllMapHandler[Model MapHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		var model Model
		result, desc, err := model.GetAllMap(core)
		if err != nil {
			respHandler.Respond(http.StatusBadRequest, 1, desc, err.Error(), true, c)
			return
		}
		if len(result) == 0 {
			respHandler.Respond(http.StatusBadRequest, 1, "PWC_WS_0008", "No Data Found", true, c)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, c)
	}
}

type GetHandler interface {
	GetSingle(
		core RequestCoreInterface,
	) (any, string, error)
	GetAll(
		core RequestCoreInterface,
	) (any, string, error)
}

func GetSingleRecord[Model GetHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		code, desc, arrayErr, model, _, err := libRequest.GetRequest[Model](w, false)
		if err != nil {
			respHandler.Respond(code, 1, desc, arrayErr, true, c)
			return
		}
		result, desc, err := model.GetSingle(core)
		if err != nil {
			respHandler.Respond(http.StatusBadRequest, 1, desc, err.Error(), true, c)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, c)
	}
}

func GetAll[Model GetHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		var model Model
		result, desc, err := model.GetAll(core)
		if err != nil {
			respHandler.Respond(http.StatusBadRequest, 1, desc, err.Error(), true, c)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, c)
	}
}

type GetPageHandler interface {
	GetPage(
		core RequestCoreInterface,
		pageSize, pageId int,
	) (any, string, error)
	GetPageParams() (int, int)
}

func GetPage[Model GetPageHandler](title string,
	core RequestCoreInterface,
	respHandler response.ResponseHandler,
	args ...any) any {
	fmt.Println("Registering: ", title)
	return func(c any) {
		w := libContext.InitContext(c)
		code, desc, arrayErr, model, _, err := libRequest.GetRequest[Model](w, false)
		if err != nil {
			respHandler.Respond(code, 1, desc, arrayErr, true, c)
			return
		}
		pageSize, pageId := model.GetPageParams()
		result, desc, err := model.GetPage(core, pageSize, pageId)
		if err != nil {
			respHandler.HandleErrorState(err, http.StatusBadRequest, desc, err.Error(), c)
			return
		}
		respHandler.Respond(http.StatusOK, 0, "OK", result, false, c)
	}
}
