package libCallApi

import (
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"
)

func (api *RemoteApi) EnsureAuthorization(w webFramework.WebFramework, headers map[string]string) libError.Error {
	if headers == nil {
		return libError.NewWithDescription(
			status.InternalServerError,
			"AUTH_HEADERS_NIL",
			"headers map is nil for api %s",
			api.Name,
		)
	}
	if _, ok := headers["Authorization"]; ok {
		return nil
	}
	if api.Auth != nil {
		if err := api.Authenticate(w); err != nil {
			return err
		}
		authHeader, err := api.GetAuthHeader()
		if err != nil {
			return libError.NewWithDescription(
				status.InternalServerError,
				"AUTH_HEADER_FAILED",
				"failed to build auth header for api %s: %v",
				api.Name,
				err,
			)
		}
		headers["Authorization"] = authHeader
		return nil
	}
	if api.AuthData.User != "" && api.AuthData.Password != "" {
		headers["Authorization"] = api.GetBasicAuthHeader()
	}
	return nil
}
