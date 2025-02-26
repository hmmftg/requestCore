package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

const (
	NO_DATA_FOUND     = "NO-DATA-FOUND"
	SYSTEM_FAULT      = "SYSTEM_FAULT"
	SYSTEM_FAULT_DESC = "خطای سیستمی"
)

func JustPrintResp(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var err error
	var resp WsRemoteResponse
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		slog.Error("error in PrintResp", slog.Any("error", err))
	}
	slog.Error("PrintResp", slog.Any("resp", resp))
	return status, nil, nil, nil
}

func ParseRemoteRespJson(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp WsRemoteResponse
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	if status != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := resp.ErrorData[0] //.(ErrorResponse)
			errorMessage := errorDesc.Description.(string)
			return status, map[string]string{"desc": errorDesc.Code, "message": errorMessage}, resp, errors.New(errorMessage)
		}
		return status, map[string]string{"desc": "Remote Resp", "message": resp.Description}, resp, errors.New(resp.Description)
	}
	return http.StatusOK, nil, resp.Result, nil
}

func ParseWsRemoteResp(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp WsRemoteResponse
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	if status != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := strings.ReplaceAll(resp.ErrorData[0].Code, "-", "_") //.(ErrorResponse)
			errorMessage := resp.ErrorData[0].Description.(string)
			return status, map[string]string{"desc": errorDesc, "message": errorMessage}, resp, errors.New(errorMessage)
		}
		return status, map[string]string{"desc": "Remote Resp", "message": resp.Description}, resp, errors.New(resp.Description)
	}
	return http.StatusOK, nil, resp, nil
}

func GetDescFromCode(code string, data any, errDescList map[string]string) (string, any) {
	if strings.Contains(code, "#") {
		code := code
		if strings.Contains(code, "-") {
			code = strings.ReplaceAll(code, "-", "_")
		}
		messageParts := strings.Split(code, "#")
		if descInDb, ok := errDescList[messageParts[0]]; ok {
			descParts := strings.Split(descInDb, "$")
			incomingDesc := messageParts[0]
			desc := ""
			//DESC_DB1 $P1$ DESC_DB2 $P2$
			//MESSAGE1#G1#G2#
			//=>
			//DESC_DB1 G1 DESC_DB2 G2
			for i, j := 0, 1; i < len(descParts); i += 2 {
				desc += descParts[i] + messageParts[j]
				j++
			}
			return strings.ReplaceAll(incomingDesc, "_", "-"), desc
		}
		return strings.ReplaceAll(code, "_", "-"), data
	}
	if desc, ok := errDescList[code]; ok {
		return strings.ReplaceAll(code, "_", "-"), desc
	}
	return code, data
}
