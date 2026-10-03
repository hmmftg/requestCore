package webFramework

import "errors"

// ErrEmptyBody is returned by RequestParser.GetBody when the request has
// no body or a body of length zero. It is distinguishable from a
// successful bind: optional binding modes (libRequest.JSONOptional and
// libRequest.JSONWithURIOptional) treat it as success, while
// required-body modes (libRequest.JSON and libRequest.JSONWithURI)
// report it as ERROR_IN_GET_REQUEST_BODY.
var ErrEmptyBody = errors.New("empty request body")
