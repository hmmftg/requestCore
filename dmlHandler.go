package requestCore

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

func PostHandler[Req libQuery.RecordDataDml](title string,
	core RequestCoreInterface,
	hasInitializer bool,
	finalizer func(request Req, c any),
	args ...any,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		code, desc, arrayErr, request, reqLog, err := libRequest.GetRequest[Req](w, true)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}

		if hasInitializer {
			w.Parser.SetLocal("reqLog", &reqLog)
			method := title
			reqLog.Incoming = request
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
				return
			}
		}

		desc, err = request.Filler(w.Parser.GetHttpHeader(), core.GetDB(), w.Parser.GetArgs(args...))
		if err != nil {
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, desc, "error in Filler", c)
			return
		}

		code, desc, err = request.CheckDuplicate(core.GetDB())
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "error in CheckDuplicate"), code, desc, "", c)
			return
		}

		code, desc, err = request.PreControl(core.GetDB())
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "error in PreControl"), code, desc, "", c)
			return
		}

		resp, code, desc, err := request.Post(core.GetDB(), w.Parser.GetArgs(args...))
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "error in Post"), code, desc, "", c)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)
		if finalizer != nil {
			finalizer(request, c)
		}
	}
}

func Dml[Req libQuery.DmlModel](
	title, key string,
	core RequestCoreInterface,
) any {
	return DmlHandler[Req](title, key, core, libRequest.JSON, true)
}

func DmlHandler[Req libQuery.DmlModel](
	title, key string,
	core RequestCoreInterface,
	mode libRequest.Type,
	validateHeader bool,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		defer func() {
			if r := recover(); r != nil {
				core.Responder().HandleErrorState(libError.Join(r.(error), "error in Dml"), http.StatusInternalServerError, response.SYSTEM_FAULT, response.SYSTEM_FAULT_DESC, c)
				panic(r)
			}
		}()
		w := libContext.InitContext(c)
		code, desc, arrayErr, request, reqLog, err := libRequest.Req[Req, libRequest.RequestHeader](w, mode, validateHeader)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}

		w.Parser.SetLocal("reqLog", &reqLog)
		method := title
		reqLog.Incoming = request
		u, _ := url.Parse(w.Parser.GetPath())
		code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
		if err != nil {
			core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
			return
		}

		preControl := request.PreControlCommands()
		for _, command := range preControl[key] {
			core.RequestTools().LogStart(w, fmt.Sprintf("PreControl: %s", command.Name), "Execute")
			_, errPreControl := command.ExecuteWithContext(w.Ctx, fmt.Sprintf("%s.%s", title, "preControl"), core.GetDB())
			if errPreControl != nil {
				core.Responder().HandleErrorState(libError.Join(errPreControl, "PreControl"), http.StatusBadRequest, errPreControl.Description, errPreControl.Message, c)
				return
			}
		}
		dml := request.DmlCommands()
		resp := map[string]any{}
		for _, command := range dml[key] {
			core.RequestTools().LogStart(w, fmt.Sprintf("Insert: %s", command.Name), "Execute")
			result, errInsert := command.ExecuteWithContext(w.Ctx, fmt.Sprintf("%s.%s", title, "dml"), core.GetDB())
			if errInsert != nil {
				core.Responder().HandleErrorState(libError.Join(errInsert, "Insert"), http.StatusBadRequest, errInsert.Description, errInsert.Message, c)
				return
			}
			resp[command.Name] = result
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)

		finalize := request.FinalizeCommands()
		for _, command := range finalize[key] {
			_, errFinalize := command.ExecuteWithContext(w.Ctx, fmt.Sprintf("%s.%s", title, "finalize"), core.GetDB())
			if errFinalize != nil {
				log.Printf("Error executing finalize command: %s=>%v", command.Name, errFinalize)
			}
		}
	}
}

func PutHandler[Req libQuery.RecordDataDml](title string,
	core RequestCoreInterface,
	hasInitializer bool,
	finalizer func(request Req, c any),
	args ...any,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		id := w.Parser.GetUrlParam("id")
		id = strings.ReplaceAll(id, "*", "/")
		code, desc, arrayErr, request, reqLog, err := libRequest.GetRequest[Req](w, true)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		request.SetId(id)

		if hasInitializer {
			w.Parser.SetLocal("reqLog", &reqLog)
			method := title
			reqLog.Incoming = request
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
				return
			}
		}

		code, desc, err = request.CheckExistence(core.GetDB())
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, "error in CheckExistence", c)
			return
		}

		resp, code, desc, err := request.Put(core.GetDB(), w.Parser.GetArgs(args...))
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, "error in Put", c)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)
		if finalizer != nil {
			finalizer(request, c)
		}
	}
}

func DeleteHandler[Req libQuery.RecordData](title, delete, checkQuery string,
	core RequestCoreInterface,
	hasInitializer bool, parser libQuery.FieldParser,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		id := w.Parser.GetUrlParam("id")
		id = strings.ReplaceAll(id, "*", "/")
		code, desc, arrayErr, reqLog, err := libRequest.GetEmptyRequest(w)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		w.Parser.SetLocal("reqLog", &reqLog)
		method := title

		u, _ := url.Parse(w.Parser.GetPath())

		if hasInitializer {
			code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
				return
			}
		}

		code, desc, data, _, err := libQuery.CallSql[libQuery.QueryData](checkQuery, core.GetDB(), id)
		if code == 400 && desc == libQuery.NO_DATA_FOUND && data == "No Data Found" {
			core.Responder().HandleErrorState(libError.Join(err, "DELETE_NOT_ALLOWED"), http.StatusBadRequest, "DELETE_NOT_ALLOWED", data, c)
			return
		}
		if err != nil {
			core.Responder().Respond(code, 1, desc, data, true, c)
			return
		}
		var request Req
		deleteParsed := w.Parser.ParseCommand(delete, title, request, parser)
		resultDb, err := core.GetDB().InsertRow(deleteParsed, id)
		if err != nil {
			core.Responder().HandleErrorState(libError.Join(err, "Exec failed"), http.StatusInternalServerError, "ERROR_CALLING_DB_FUNCTION", resultDb, c)
			return
		}

		var resp libQuery.DmlResult
		resp.LastInsertId, _ = resultDb.LastInsertId()
		resp.RowsAffected, _ = resultDb.RowsAffected()

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)
	}
}

func UpdateHandler[Req libQuery.Updatable](title string, hasReqLog bool,
	core RequestCoreInterface,
	hasInitializer bool,
	finalizer func(request Req, c any),
	args ...string,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		params := make(map[string]string, 0)
		for _, arg := range args {
			val, exists := w.Parser.CheckUrlParam(arg)
			if exists {
				params[arg] = val
			}
		}
		code, desc, arrayErr, request, reqLog, err := libRequest.GetRequest[Req](w, true)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, arrayErr, c)
			return
		}
		if hasReqLog {
			w.Parser.SetLocal("reqLog", &reqLog)
		}
		filledRequest := request.SetParams(params).(Req)
		method := title
		reqLog.Incoming = filledRequest

		if hasInitializer {
			u, _ := url.Parse(w.Parser.GetPath())
			code, result, err := core.RequestTools().Initialize(w, method, u.Path, &reqLog)
			if err != nil {
				core.Responder().HandleErrorState(err, code, result["desc"], result["message"], c)
				return
			}
		}

		code, desc, data, _, err := libQuery.CallSql[libQuery.QueryData](filledRequest.GetCountCommand(), core.GetDB(), filledRequest.GetUniqueId()...)
		if err != nil {
			core.Responder().HandleErrorState(err, code, desc, data, c)
			return
		}

		if desc == libQuery.NO_DATA_FOUND {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, c)
			return
		}

		update, updateArgs := filledRequest.GetUpdateCommand()
		resultDb, err := core.GetDB().InsertRow(update, updateArgs...)
		if err != nil {
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, "ERROR_CALLING_DB_FUNCTION", arrayErr, c)
			return
		}

		var resp libQuery.DmlResult
		resp.LastInsertId, _ = resultDb.LastInsertId()
		resp.RowsAffected, _ = resultDb.RowsAffected()

		if resp.RowsAffected == 0 {
			core.Responder().HandleErrorState(fmt.Errorf(libQuery.NO_DATA_FOUND), http.StatusBadRequest, libQuery.NO_DATA_FOUND, arrayErr, c)
			return
		}

		desc, err = filledRequest.Finalize(core.GetDB())
		if err != nil {
			core.Responder().HandleErrorState(err, http.StatusInternalServerError, desc, arrayErr, c)
			return
		}

		core.Responder().Respond(http.StatusOK, 0, "OK", resp, false, c)
		if finalizer != nil {
			finalizer(request, c)
		}
	}
}
