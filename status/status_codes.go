package status

import (
	"fmt"
	"net/http"
)

type StatusCode int

const (
	OK                  StatusCode = http.StatusOK
	Unknown             StatusCode = http.StatusInternalServerError
	InternalServerError StatusCode = http.StatusInternalServerError
	BadRequest          StatusCode = http.StatusBadRequest
	DuplicateRequest    StatusCode = http.StatusTooManyRequests
	NotFound            StatusCode = http.StatusNotFound
)

func (s StatusCode) String() string {
	return fmt.Sprintf("%d-%s", s, http.StatusText(int(s)))
}

func (s StatusCode) Int() int {
	return int(s)
}
