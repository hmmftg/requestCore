package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

func ConsumeRemoteGet[Result any](
	w webFramework.WebFramework,
	api, url string,
	core requestCore.RequestCoreInterface,
	args ...any) (*Result, response.ErrorState) {
	var params []any
	for _, arg := range args {
		switch val := arg.(type) {
		case string:
			if val == "QUERY" {
				continue
			}
			argString := val
			if len(w.Parser.GetUrlParam(argString)) > 0 {
				params = append(params, w.Parser.GetUrlParam(argString))
			} else {
				if strings.Contains(argString, ":") {
					argSplit := strings.Split(argString, ":")
					switch argSplit[0] {
					case "db":
						_, _, _, argDb, err := libQuery.CallSql[libQuery.QueryData](argSplit[1], core.GetDB())
						if err != nil {
							return nil, response.Error(
								http.StatusBadRequest,
								"UNABLE_TO_PARSE_DB_ARG",
								"unable to parse db argument",
								err,
							)
						}
						params = append(params, argDb[0].Value)
					case "consume":
						consumeArgs := strings.Split(argSplit[1], ",")
						// 200, 0, "OK", resp.Result, false, nil
						remoteMap, err := ConsumeRemoteGet[map[string]any](w, consumeArgs[0], consumeArgs[1], core, consumeArgs[2])
						if err != nil {
							return nil, response.Error(
								http.StatusBadRequest,
								"UNABLE_TO_PARSE_DB_ARG",
								"unable to parse db argument",
								err,
							)
						}
						params = append(params, (*remoteMap)[consumeArgs[3]])
					}
				} else {
					params = append(params, w.Parser.GetLocalString(argString))
				}
			}
		}
	}
	path := fmt.Sprintf(url, params...)

	reqLog := core.RequestTools().LogStart(w, "ConsumeRemoteGet", path)
	headersMap := extractHeaders(w, DefaultHeaders(), DefaultLocals())

	respBytes, desc, err := core.Consumer().ConsumeRestBasicAuthApi(nil, api, path, "application/x-www-form-urlencoded", "GET", headersMap)
	if err != nil {
		return nil, response.Error(
			http.StatusInternalServerError,
			desc,
			string(respBytes),
			err,
		)
	}
	core.RequestTools().LogEnd("ConsumeRemoteGet", desc, reqLog)

	var resp WsResponse[Result]
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		return nil, response.Error(
			http.StatusInternalServerError,
			"invalid resp from "+api, err.Error(),
			err,
		)
	}
	stat, err := strconv.Atoi(strings.Split(desc, " ")[0])
	if stat != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := resp.ErrorData // .(requestCore.ErrorResponse)
			err = errors.New(errorDesc[0].Code)
			if errorDesc[0].Description != nil {
				switch v := errorDesc[0].Description.(type) {
				case string:
					err = errors.New(v)
				}
			}
			return nil, response.Error(
				stat,
				errorDesc[0].Code,
				errorDesc[0].Description,
				err,
			)
		}
		if len(resp.Description) > 0 {
			return nil, response.Error(
				stat,
				api+" Resp", resp.Description,
				err,
			)
		}
		return nil, response.Error(
			http.StatusInternalServerError,
			"invalid resp from "+api, err.Error(),
			err,
		)
	}

	return &resp.Result, nil
}

func ConsumeRemoteGetApi[Result any](
	api, url string,
	core requestCore.RequestCoreInterface,
	args ...any) any {
	log.Println("ConsumeRemoteGetApi...")
	return func(c context.Context) {
		w := libContext.InitContextNoAuditTrail(c)
		fullPath := url
		if len(args) > 0 && args[0] == "QUERY" {
			fullPath = fmt.Sprintf("%s?%s", fullPath, w.Parser.GetRawUrlQuery())
		}

		result, err := ConsumeRemoteGet[Result](w, api, fullPath, core, args...)
		if err != nil {
			core.Responder().Error(w, err)
			return
		}
		core.Responder().OK(w, result)
	}
}
