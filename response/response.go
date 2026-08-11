package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

const (
	// NoDataFound is the error code for missing data.
	NoDataFound = "NO-DATA-FOUND"
	// SystemFault is the error code for internal system faults.
	SystemFault = "SYSTEM_FAULT"
	// SystemFaultDesc is the default localized description for system faults.
	SystemFaultDesc = "خطای سیستمی"
)

// JustPrintResp unmarshals and logs a remote response without returning parsed data.
func JustPrintResp(respBytes []byte, _ string, status int) (int, map[string]string, any, error) {
	var err error
	var resp WsRemoteResponse
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		slog.Error("error in PrintResp", slog.Any("error", err))
	}
	slog.Error("PrintResp", slog.Any("resp", resp))
	return status, nil, nil, nil
}

// ParseRemoteRespJSON parses a remote JSON response and extracts status, error details, and result.
func ParseRemoteRespJSON(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp WsRemoteResponse
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	if status != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := resp.ErrorData[0]
			return status, map[string]string{"desc": errorDesc.Code, "message": errorDesc.Description}, resp, errors.New(errorDesc.Description)
		}
		return status, map[string]string{"desc": "Remote Resp", "message": resp.Description}, resp, errors.New(resp.Description)
	}
	return http.StatusOK, nil, resp.Result, nil
}

// ParseWsRemoteResp parses a remote JSON response and returns the full WsRemoteResponse on success.
func ParseWsRemoteResp(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp WsRemoteResponse
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	if status != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := strings.ReplaceAll(resp.ErrorData[0].Code, "-", "_")
			errorMessage := resp.ErrorData[0].Description
			return status, map[string]string{"desc": errorDesc, "message": errorMessage}, resp, errors.New(errorMessage)
		}
		return status, map[string]string{"desc": "Remote Resp", "message": resp.Description}, resp, errors.New(resp.Description)
	}
	return http.StatusOK, nil, resp, nil
}

// GetDescFromCode returns (code, description) for API response. When code is not in errDescList,
// it returns a safe fallback (SystemFault + localized text) and never exposes raw data.
func GetDescFromCode(code string, _ any, errDescList map[string]string) (string, string) {
	safeFallbackDesc := SystemFaultDesc
	if d, ok := errDescList[SystemFault]; ok {
		safeFallbackDesc = d
	}
	if strings.Contains(code, "#") {
		codeNorm := code
		if strings.Contains(codeNorm, "-") {
			codeNorm = strings.ReplaceAll(codeNorm, "-", "_")
		}
		messageParts := strings.Split(codeNorm, "#")
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
			return strings.ReplaceAll(incomingDesc, "_", "-"), SanitizeForClient(desc, MaxDescriptionLength)
		}
		return strings.ReplaceAll(codeNorm, "_", "-"), safeFallbackDesc
	}
	if desc, ok := errDescList[code]; ok {
		return strings.ReplaceAll(code, "_", "-"), SanitizeForClient(desc, MaxDescriptionLength)
	}
	return strings.ReplaceAll(code, "_", "-"), safeFallbackDesc
}
