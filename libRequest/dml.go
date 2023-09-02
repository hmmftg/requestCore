package libRequest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hmmftg/requestCore/libError"
)

func (m RequestModel) CheckDuplicateRequest(request Request) error {
	ret, result, err := m.QueryInterface.QueryRunner(m.QueryInDb, request.Header.GetId())
	if err != nil {
		return err
	}
	if ret != 0 {
		if len(result) > 0 {
			return fmt.Errorf("query(%s)=>%d", request.Header.GetId(), ret)
		}
		return fmt.Errorf("query(%s)=>%d,%s", request.Header.GetId(), ret, result[0])
	}
	if len(result) > 0 {
		return fmt.Errorf("duplicate Request: id: %s", request.Header.GetId())
	}
	return nil
}

func (m RequestModel) InsertRequest(request Request) error {
	return m.InsertRequestWithContext(context.Background(), request)
}

func (m RequestModel) InsertRequestWithContext(ctx context.Context, request Request) error {
	rowByte, err := json.Marshal(request)
	if err != nil {
		return err
	}
	args := []any{string(rowByte)}
	if strings.Contains(m.InsertInDb, "$1") {
		args = append(args, request.Req)
	}
	ret, err := m.QueryInterface.Dml(ctx, "InsertRequest",
		m.InsertInDb,
		args...,
	)
	if err != nil {
		return libError.Join(err, "error in InsertNewRequest[Dml](%v)=>%v", args, ret)
	}
	return nil
}

func (m RequestModel) UpdateRequest(request Request) error {
	return m.UpdateRequestWithContext(context.Background(), request)
}

func (m RequestModel) UpdateRequestWithContext(ctx context.Context, request Request) error {
	requestBytes, _ := json.Marshal(request)
	args := []any{string(requestBytes)}
	args = append(args, request.Id)
	if strings.Contains(m.UpdateInDb, "$3") || strings.Contains(m.UpdateInDb, ":3") {
		args = append(args, request.Resp)
	}

	ret, err := m.QueryInterface.Dml(ctx, "UpdateRequest",
		m.UpdateInDb,
		args...,
	)
	if err != nil {
		return libError.Join(err, "error in UpdateRequest[Dml]()=>%v", ret)
	}

	return nil
}
