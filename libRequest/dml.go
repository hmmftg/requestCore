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

// CheckDuplicateRequest queries the database to verify the request ID is not a duplicate.
func (m RequestModel) CheckDuplicateRequest(request RequestPtr) error {
	result, err := libQuery.GetQuery[RequestPtr](m.QueryInDb, m.QueryInterface, request.Header.GetID())
	if err != nil {
		if ok, err := libError.Unwrap(err); ok && err.Action().Description == libQuery.NoDataFound {
			return nil
		}
		return errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"ERROR_IN_CHECK_DUPLICATE_REQUEST",
				"duplicate Request: id: %s", request.Header.GetID(),
			))
	}
	if len(result) > 0 {
		return errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				libQuery.DuplicateFound,
				"duplicate Request: id: %s", request.Header.GetID(),
			))
	}
	return nil
}

// InsertRequest inserts a request into the database using a background context.
func (m RequestModel) InsertRequest(request RequestPtr) error {
	return m.InsertRequestWithContext(context.Background(), request)
}

// ModuleName is the module name used for request handler DML operations.
const ModuleName = "RequestHandler"

// InsertRequestWithContext inserts a request into the database using the given context.
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

// UpdateRequest updates a request in the database using a background context.
func (m RequestModel) UpdateRequest(request RequestPtr) error {
	return m.UpdateRequestWithContext(context.Background(), request)
}

// UpdateRequestWithContext updates a request in the database using the given context.
func (m RequestModel) UpdateRequestWithContext(ctx context.Context, request RequestPtr) error {
	requestBytes, _ := json.Marshal(request)
	args := []any{string(requestBytes)}
	args = append(args, request.ID)
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
