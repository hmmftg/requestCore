package libRequest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
)

func (m RequestModel) CheckDuplicateRequest(request RequestPtr) error {
	result, err := libQuery.GetQuery[RequestPtr](m.QueryInDb, m.QueryInterface, request.Header.GetId())
	if err != nil {
		if ok, err := libError.Unwrap(err); ok && err.Action().Description == libQuery.NO_DATA_FOUND {
			return nil
		}
		return errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"ERROR_IN_CHECK_DUPLICATE_REQUEST",
				"duplicate Request: id: %s", request.Header.GetId(),
			))
	}
	if len(result) > 0 {
		return errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				libQuery.DUPLICATE_FOUND,
				"duplicate Request: id: %s", request.Header.GetId(),
			))
	}
	return nil
}

func (m RequestModel) InsertRequest(request RequestPtr) error {
	return m.InsertRequestWithContext(context.Background(), request)
}

const ModuleName = "RequestHandler"

func (m RequestModel) InsertRequestWithContext(ctx context.Context, request RequestPtr) error {
	rowByte, err := json.Marshal(request)
	if err != nil {
		return response.ToErrorState(err)
	}
	args := []any{string(rowByte)}
	if strings.Contains(m.InsertInDb, "$2") {
		args = append(args, request.Req)
	}
	ret, err := m.QueryInterface.Dml(ctx, ModuleName, "InsertRequest",
		m.InsertInDb,
		args...,
	)
	if err != nil {
		return response.ToErrorState(libError.Join(err, "error in InsertNewRequest[Dml](%v)=>%v,%v", args, ret, err))
	}
	return nil
}

func (m RequestModel) UpdateRequest(request RequestPtr) error {
	return m.UpdateRequestWithContext(context.Background(), request)
}

func (m RequestModel) UpdateRequestWithContext(ctx context.Context, request RequestPtr) error {
	requestBytes, _ := json.Marshal(request)
	args := []any{string(requestBytes)}
	args = append(args, request.Id)
	if strings.Contains(m.UpdateInDb, "$3") || strings.Contains(m.UpdateInDb, ":3") {
		args = append(args, request.Resp)
	}

	ret, err := m.QueryInterface.Dml(ctx, ModuleName, "UpdateRequest",
		m.UpdateInDb,
		args...,
	)
	if err != nil {
		return response.ToErrorState(libError.Join(err, "error in UpdateRequest[Dml]()=>%v,%v", ret, err))
	}

	m.QueryInterface.Close()

	return nil
}
