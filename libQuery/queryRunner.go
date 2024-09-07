package libQuery

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"slices"
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

func (m QueryRunnerModel) NewStatement(command string) (*sql.Stmt, error) {
	errPing := m.DB.Ping()
	if errPing != nil {
		log.Println("error in ping", errPing)
	}
	stmt, err := m.DB.Prepare(command)
	if err != nil {
		return nil, libError.Join(err, "QueryRunner[prepare](%s)", command)
	}
	return stmt, nil
}

func (m QueryRunnerModel) QueryRunner(querySql string, args ...any) (int, []map[string]any, error) {
	errPing := m.DB.Ping()
	if errPing != nil {
		log.Println("error in ping", errPing)
	}
	stmt, err := m.DB.Prepare(querySql)
	finalRows := []map[string]any{}
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

	baseArgs := make([]any, count)

	for i := range columnTypes {
		baseArgs[i] = new(sql.Null[any])
	}

	for rows.Next() {
		scanArgs := slices.Clone(baseArgs)
		err := rows.Scan(scanArgs...)

		if err != nil {
			errorData["step"] = "column scan"
			finalRows = append(finalRows, errorData)
			return SCAN_ERROR, finalRows, libError.Join(err, "QueryRunner[Scan](%s,%v)", querySql, scanArgs)
		}

		masterData := map[string]any{}

		for i, v := range columnTypes {
			masterData[v.Name()] = scanArgs[i].(*sql.Null[any]).V
		}

		finalRows = append(finalRows, masterData)
	}
	//resp, _ := json.Marshal(finalRows)
	return 0, finalRows, nil
}

func QueryToStruct[Target any](q QueryRunnerInterface, querySql string, args ...any) ([]Target, error) {
	stmt, err := q.NewStatement(querySql)
	if err != nil {
		return nil, libError.Join(err, "QueryRunner[prepare](%s,%v)", querySql, args)
	}
	defer stmt.Close()
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, libError.Join(err, "QueryRunner[query](%s,%v)", querySql, args)
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, libError.Join(err, "QueryRunner[ColumnTypes](%s,%v)", querySql, args)
	}

	count := len(columnTypes)

	baseArgs := make([]any, count)

	for i := range columnTypes {
		baseArgs[i] = new(sql.Null[any])
	}

	finalRows := make([]Target, 0)

	for rows.Next() {
		scanArgs := slices.Clone(baseArgs)
		err := rows.Scan(scanArgs...)

		if err != nil {
			return nil, libError.Join(err, "QueryRunner[Scan](%s,%v)", querySql, scanArgs)
		}

		masterData := map[string]any{}

		for i, v := range columnTypes {
			masterData[v.Name()] = scanArgs[i].(*sql.Null[any]).V
		}

		parsed, err := ParseMap[Target](masterData)
		if parsed == nil {
			return nil, libError.Join(err, "QueryRunner[parse](unable to parse: %v)", masterData)
		}
		finalRows = append(finalRows, *parsed)
	}
	//resp, _ := json.Marshal(finalRows)
	return finalRows, nil
}

func GetTagValue(name, tag string, s any) (*string, *string, error) {
	//var elemType reflect.Type
	elemType := reflect.TypeOf(s)
	elemValue := reflect.ValueOf(s)
	if elemType.Kind() == reflect.Pointer {
		// TODO handle interface type
		elemType = elemType.Elem()
		elemValue = elemValue.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		if elemType.Kind() == reflect.Interface {
			// TODO handle interface type
			// pt := reflect.ValueOf(s).Elem()
			return nil, nil, fmt.Errorf("bad type, %T is interface not struct", s)

		}
		return nil, nil, fmt.Errorf("bad type, %T is not struct", s)
	}
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		tagID := strings.Split(f.Tag.Get(tag), ",")[0] // use split to ignore tag "options" like omitempty, etc.
		if tagID == name {
			switch elemValue.Field(i).Kind() {
			case reflect.String:
				v := elemValue.Field(i).String()
				return &f.Name, &v, nil
			case reflect.Int64:
				i := elemValue.Field(i).Int()
				v := fmt.Sprintf("%d", i)
				return &f.Name, &v, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("field %s with tag %s is not present in %T ", name, tag, s)
}

func GetDBTagValue(name string, s any) (*string, *string, error) {
	return GetTagValue(name, "db", s)
}

func GetFormTagValue(name string, s any) (*string, *string, error) {
	return GetTagValue(name, "form", s)
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
		ParseQueryResult(row, elemType, newRow)
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
		return nil, response.Error(http.StatusBadRequest, NO_DATA_FOUND, "No Data Found", err)
	}
	if nRet != 0 || err != nil {
		return nil, response.ToError(DB_READ_ERROR, err.Error(), err)
	}
	rows, ok := result.([]R)
	if !ok {
		return nil, response.ToError(PARSE_DB_RESP_ERROR, "Unable to parse response struct", err)
	}
	if len(rows) == 0 {
		return nil, response.Error(
			http.StatusBadRequest,
			NO_DATA_FOUND,
			"No Data Found",
			fmt.Errorf("no data found: %s,%v", query, args),
		)
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
	Transforms
)

func Query[Result QueryResult](core QueryRunnerInterface, command QueryCommand, args ...any) (any, response.ErrorState) {
	switch command.Type {
	case QuerySingle:
		_, desc, data, resp, err := CallSql[Result](command.Command, core, args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "QuerySingle: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.Error(
				http.StatusBadRequest,
				NO_DATA_FOUND,
				NO_DATA_FOUND_DESC,
				fmt.Errorf("QuerySingle: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		return resp[0], nil
	case QueryAll:
		_, desc, data, resp, err := CallSql[Result](command.Command, core, args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "QueryAll: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.Error(
				http.StatusBadRequest,
				NO_DATA_FOUND,
				NO_DATA_FOUND_DESC,
				fmt.Errorf("QueryAll: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		return resp, nil
	case QueryMap:
		_, desc, data, resp, err := CallSql[Result](command.Command, core, args...)
		if err != nil {
			return nil, response.ToError(desc, data, libError.Join(err, "QueryMap: %s", command.Name))
		}
		if desc == NO_DATA_FOUND {
			return nil, response.Error(
				http.StatusBadRequest,
				NO_DATA_FOUND,
				NO_DATA_FOUND_DESC,
				fmt.Errorf("QueryMap: %s=> %s", command.Name, NO_DATA_FOUND))
		}
		respMap := make(map[string]any)
		for _, record := range resp {
			respMap[record.GetID()] = record.GetValue()
		}
		return respMap, nil
	}
	return nil, nil
}
