package libQuery

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/response"
)

func (m QueryRunnerModel) GetModule() (string, string) {
	return m.ModuleName, m.ProgramName
}

const (
	PREPARE_ERROR = -1
	QUERY_ERROR   = -2
	PARSE_ERROR   = -3
	SCAN_ERROR    = -4
)

func (m QueryRunnerModel) QueryRunner(querySql string, args ...any) (int, []any, error) {
	errPing := m.DB.Ping()
	if errPing != nil {
		log.Println("error in ping", errPing)
	}
	stmt, err := m.DB.Prepare(querySql)
	finalRows := []any{}
	errorData := map[string]any{}
	if err != nil {
		errorData["step"] = "prepare"
		finalRows = append(finalRows, errorData)
		return PREPARE_ERROR, finalRows, libError.Join(err, "QueryRunner[prepare](%s,%v)", querySql, args)
	}
	defer stmt.Close()
	rows, err := stmt.Query(args...)
	if err != nil {
		errorData["step"] = "query"
		finalRows = append(finalRows, errorData)
		return QUERY_ERROR, finalRows, libError.Join(err, "QueryRunner[query](%s,%v)", querySql, args)
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		errorData["step"] = "column types"
		finalRows = append(finalRows, errorData)
		return PARSE_ERROR, finalRows, libError.Join(err, "QueryRunner[ColumnTypes](%s,%v)", querySql, args)
	}

	count := len(columnTypes)

	for rows.Next() {

		scanArgs := make([]any, count)

		for i, v := range columnTypes {

			switch v.DatabaseTypeName() {
			case "NCHAR", "VARCHAR", "_VARCHAR", "TEXT", "UUID", "TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE", "JSON", "CHAR", "DATE", "ROWID":
				scanArgs[i] = new(sql.NullString)
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
			case "INT4", "INT64", "NUMBER":
				scanArgs[i] = new(sql.NullInt64)
			default:
				//log.Println("Undefined Type Name: ", v.DatabaseTypeName())
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)

		if err != nil {
			errorData["step"] = "column scan"
			finalRows = append(finalRows, errorData)
			return SCAN_ERROR, finalRows, libError.Join(err, "QueryRunner[Scan](%s,%v)", querySql, scanArgs)
		}

		masterData := map[string]any{}

		for i, v := range columnTypes {

			if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
				masterData[v.Name()] = z.Bool
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				masterData[v.Name()] = z.String
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				masterData[v.Name()] = z.Int64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				masterData[v.Name()] = z.Float64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
				masterData[v.Name()] = z.Int32
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}
	//resp, _ := json.Marshal(finalRows)
	return 0, finalRows, nil
}

func (m QueryRunnerModel) QueryToStruct(querySql string, target any, args ...any) (int, any, error) {
	ret, results, err := m.QueryRunner(querySql, args...)
	if err != nil {
		return ret, nil, err
	}

	var elemType reflect.Type
	if reflect.TypeOf(target).Kind() == reflect.Pointer {
		elemType = reflect.TypeOf(target).Elem()
	} else {
		elemType = reflect.TypeOf(target)
	}
	elemSlice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(results))
	for _, row := range results {
		newRow := reflect.New(elemType).Elem()
		ParseQueryResult(row.(map[string]any), elemType, newRow)
		elemSlice = reflect.Append(elemSlice, newRow)
	}

	return 0, elemSlice.Interface(), nil
}

const (
	NO_DATA_FOUND        = "NO_DATA_FOUND"
	NO_DATA_FOUND_DESC   = "رکوردی یافت نشد"
	DUPLICATE_FOUND      = "DUPLICATE_FOUND"
	DUPLICATE_FOUND_DESC = "رکورد تکراری است"
	DB_READ_ERROR        = "DB_READ_ERROR"
	PARSE_DB_RESP_ERROR  = "PARSE_DB_RESP_ERROR"
)

func GetQueryResp[R any](query string, core QueryRunnerInterface, args ...any) (int, string, string, bool, any, error) {
	//Query
	var target R
	nRet, result, err := core.QueryToStruct(query, target, args...)
	if nRet == QUERY_ERROR && strings.HasPrefix(err.Error(), "no data found") {
		return http.StatusBadRequest, NO_DATA_FOUND, "No Data Found", true, nil, err
	}
	if nRet != 0 || err != nil {
		return http.StatusInternalServerError, DB_READ_ERROR, err.Error(), true, nil, err
	}
	if err != nil {
		return http.StatusBadRequest, PARSE_DB_RESP_ERROR, "Unable to parse response", true, nil, err
	}
	return http.StatusOK, "", "", false, result, nil
}

func CallSql[R any](query string,
	core QueryRunnerInterface,
	args ...any) (int, string, string, []R, error) {
	code, desc, data, _, resultQuery, err := GetQueryResp[R](query, core, args...)
	if err != nil {
		return code, desc, data, nil, err
	}
	array := resultQuery.([]R)
	if len(array) == 0 {
		return http.StatusBadRequest, NO_DATA_FOUND, "No Data Found", nil, fmt.Errorf("no data found: %s,%v", query, args)
	}
	return http.StatusOK, desc, data, array, nil
}

func GetQuery[R any](query string, core QueryRunnerInterface, args ...any) ([]R, response.ErrorState) {
	//Query
	var target R
	nRet, result, err := core.QueryToStruct(query, target, args...)
	if nRet == QUERY_ERROR && strings.HasPrefix(err.Error(), "no data found") {
		return nil, response.ToError(NO_DATA_FOUND, "No Data Found", err)
	}
	if nRet != 0 || err != nil {
		return nil, response.ToError(DB_READ_ERROR, err.Error(), err)
	}
	if err != nil {
		return nil, response.ToError(PARSE_DB_RESP_ERROR, "Unable to parse response", err)
	}
	rows, ok := result.([]R)
	if !ok {
		return nil, response.ToError(PARSE_DB_RESP_ERROR, "Unable to parse response struct", err)
	}
	if len(rows) == 0 {
		return nil, response.ToError(NO_DATA_FOUND, "No Data Found", fmt.Errorf("no data found: %s,%v", query, args))
	}
	return rows, nil
}

func Filler[Data any](
	query string,
	core QueryRunnerInterface,
	args ...any,
) (Data, string, string, error) {
	var d Data
	_, desc, data, resultD, err := CallSql[Data](query, core, args...)
	if err != nil {
		return d, desc, data, err
	}
	return resultD[0], "", "", nil
}
func Fill[Data any](
	query string,
	core QueryRunnerInterface,
	args ...any,
) ([]Data, string, string, error) {
	var d []Data
	_, desc, data, resultD, err := CallSql[Data](query, core, args...)
	if err != nil {
		return d, desc, data, err
	}
	return resultD, "", "", nil
}

const (
	QuerySingle QueryCommandType = iota
	QueryAll
	QueryMap
)

func Query[Result QueryResult](core QueryRunnerInterface, command QueryCommand, args ...any) (any, response.ErrorState) {
	switch command.Type {
	case QuerySingle:
		_, desc, data, resp, err := CallSql[Result](command.Command, core, args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "QuerySingle: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.ToError(NO_DATA_FOUND, NO_DATA_FOUND_DESC, fmt.Errorf("QuerySingle: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		return resp[0], nil
	case QueryAll:
		_, desc, data, resp, err := CallSql[Result](command.Command, core, args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "QueryAll: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.ToError(NO_DATA_FOUND, NO_DATA_FOUND_DESC, fmt.Errorf("QueryAll: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		return resp, nil
	case QueryMap:
		_, desc, data, resp, err := CallSql[Result](command.Command, core, args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "QueryMap: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.ToError(NO_DATA_FOUND, NO_DATA_FOUND_DESC, fmt.Errorf("QueryMap: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		respMap := make(map[string]any)
		for _, record := range resp {
			respMap[record.GetID()] = record.GetValue()
		}
		return respMap, nil
	}
	return nil, nil
}
