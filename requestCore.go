// Package requestCore provides the core request processing pipeline framework.
package requestCore

import (
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

// GetDB returns the query runner interface for database access.
func (m RequestCoreModel) GetDB() libQuery.QueryRunnerInterface {
	return m.QueryInterface
}

// ORM returns the ORM interface for Gorm-based database access.
func (m RequestCoreModel) ORM() liborm.OrmInterface {
	return m.OrmInterface
}

// RequestTools returns the request interface for request lifecycle management.
func (m RequestCoreModel) RequestTools() libRequest.RequestInterface {
	return m.RequestInterface
}

// Responder returns the response handler for formatting HTTP responses.
func (m RequestCoreModel) Responder() response.ResponseHandler {
	return m.RespHandler
}

// Params returns the application parameter interface.
func (m RequestCoreModel) Params() libParams.ParamInterface {
	return m.ParamMap
}
