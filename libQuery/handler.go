package libQuery

import (
	"fmt"
	"net/http"
)

func HandleCheckDuplicate(code int, desc, dupDesc string, record []QueryData, err error) (int, string, error) {
	if desc != NO_DATA_FOUND && len(record) != 0 {
		return http.StatusBadRequest, dupDesc, fmt.Errorf("check duplicate faile: %s", dupDesc)
	}
	if desc != NO_DATA_FOUND && err != nil {
		return code, desc, err
	}
	return http.StatusOK, "", nil
}

func HandleCheckExistence(code int, desc, notExistDesc string, record []QueryData, err error) (int, string, error) {
	if err != nil {
		if desc == NO_DATA_FOUND || len(record) == 0 {
			return http.StatusBadRequest, notExistDesc, fmt.Errorf("check existance faile: %s", notExistDesc)
		}
		return code, desc, err
	}
	return http.StatusOK, "", nil
}

func (m QueryRunnerModel) Close() {
	if m.Mode == MockDB {
		m.DB.Close()
	}
}
