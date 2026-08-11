// Package libQuery provides database query runner abstractions for requestCore.
package libQuery

import (
	"fmt"
	"net/http"
)

// HandleCheckDuplicate checks query results for duplicate records and returns the appropriate status.
func HandleCheckDuplicate(code int, desc, dupDesc string, record []QueryData, err error) (int, string, error) {
	if desc != NoDataFound && len(record) != 0 {
		return http.StatusBadRequest, dupDesc, fmt.Errorf("check duplicate faile: %s", dupDesc)
	}
	if desc != NoDataFound && err != nil {
		return code, desc, err
	}
	return http.StatusOK, "", nil
}

// HandleCheckExistence checks query results for record existence and returns the appropriate status.
func HandleCheckExistence(code int, desc, notExistDesc string, record []QueryData, err error) (int, string, error) {
	if err != nil {
		if desc == NoDataFound || len(record) == 0 {
			return http.StatusBadRequest, notExistDesc, fmt.Errorf("check existence failed: %s", notExistDesc)
		}
		return code, desc, err
	}
	return http.StatusOK, "", nil
}

// Close closes the database connection for mock DB mode.
func (m QueryRunnerModel) Close() {
	if m.Mode == MockDB {
		_ = m.DB.Close()
	}
}
