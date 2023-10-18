package libRequest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
)

func (m RequestModel) CheckDuplicateRequest(request RequestPtr) response.ErrorState {
	ret, result, err := m.QueryInterface.QueryRunner(m.QueryInDb, request.Header.GetId())
	if err != nil {
		return response.ToErrorState(err).Input("DB")
	}
	if ret != 0 {
		if len(result) > 0 {
			return response.ToErrorState(fmt.Errorf("query(%s)=>%d", request.Header.GetId(), ret)).Input("COUNT1")
		}
		return response.ToErrorState(fmt.Errorf("query(%s)=>%d,%s", request.Header.GetId(), ret, result[0])).Input("RETURN")
	}
	if len(result) > 0 {
		return response.ToErrorState(fmt.Errorf("duplicate Request: id: %s", request.Header.GetId())).Input("COUNT2")
	}
	return nil
}

func (m RequestModel) InsertRequest(request RequestPtr) response.ErrorState {
	return m.InsertRequestWithContext(context.Background(), request)
}

const ModuleName = "RequestHandler"

func (m RequestModel) InsertRequestWithContext(ctx context.Context, request RequestPtr) response.ErrorState {
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

func (m RequestModel) UpdateRequest(request RequestPtr) response.ErrorState {
	return m.UpdateRequestWithContext(context.Background(), request)
}

func (m RequestModel) UpdateRequestWithContext(ctx context.Context, request RequestPtr) response.ErrorState {
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
