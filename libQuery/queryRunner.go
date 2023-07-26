package libQuery

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/valyala/fasttemplate"
)

func ParseQueryResult(result map[string]any, t reflect.Type, v reflect.Value) {
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		switch result[tag].(type) {
		case string:
			value := result[tag].(string)
			if value == "true" || value == "false" {
				bl, _ := strconv.ParseBool(value)
				v.FieldByName(t.Field(i).Name).SetBool(bl)
			} else if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
				values := strings.Split(value[1:len(value)-1], ",")
				slice := reflect.MakeSlice(reflect.TypeOf([]string{}), 0, 0)
				for _, member := range values {
					newSlice := reflect.Append(slice, reflect.ValueOf(member))
					v.FieldByName(t.Field(i).Name).Set(newSlice)
					slice = v.FieldByName(t.Field(i).Name)
				}
			} else {
				if t.Field(i).Type.Kind() == reflect.Slice {
					v.FieldByName(t.Field(i).Name).Set(reflect.MakeSlice(reflect.TypeOf([]string{}), 0, 0))
				} else {
					v.FieldByName(t.Field(i).Name).SetString(value)
				}
			}
		case bool:
			v.FieldByName(t.Field(i).Name).SetBool(result[tag].(bool))
		case int64:
			v.FieldByName(t.Field(i).Name).SetInt(result[tag].(int64))
		case float64:
			v.FieldByName(t.Field(i).Name).SetFloat(result[tag].(float64))
		}
	}
}

func (m QueryRunnerModel) GetModule() (string, string) {
	return m.ModuleName, m.ProgramName
}

func (m QueryRunnerModel) InsertRow(insert string, args ...any) (sql.Result, error) {
	result, err := m.DB.Exec(
		insert,
		args...)
	if err != nil {
		return nil, libError.Join(err, "InsertRow(%s,%s)=>%v", insert, args, result)
	}
	return result, nil
}

func (m QueryRunnerModel) CallDbFunction(callString string, args ...any) (int, string, error) {
	_, err := m.DB.Exec(callString, args...)
	if err != nil {
		return -3, "ERROR_CALLING_DB_FUNCTION", libError.Join(err, "CallDbFunction[Exec](%s,%v)", callString, args)
	}

	return 0, "OK", nil
}

const (
	PREPARE_ERROR = -1
	QUERY_ERROR   = -2
	PARSE_ERROR   = -3
	SCAN_ERROR    = -4
)

func (m QueryRunnerModel) QueryRunner(querySql string, args ...any) (int, []any, error) {
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
			case "INT4", "NUMBER":
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

func ConvertJsonToStruct[Q any](row string) (Q, error) {
	var rowObj Q
	err := json.Unmarshal([]byte(row), &rowObj)
	if err != nil {
		return rowObj, err
	}

	return rowObj, nil
}

func GetQueryResp[R any](query string, core QueryRunnerInterface, args ...any) (int, string, string, bool, any, error) {
	//Query
	var target R
	nRet, result, err := core.QueryToStruct(query, target, args...)
	if nRet == QUERY_ERROR || strings.HasPrefix(err.Error(), "no data found") {
		return http.StatusBadRequest, NO_DATA_FOUND, "No Data Found", true, nil, err
	}
	if nRet != 0 || err != nil {
		return http.StatusInternalServerError, "DB_READ_ERROR", err.Error(), true, nil, err
	}
	if err != nil {
		return http.StatusBadRequest, "", "Unable to parse response", true, nil, err
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

type QueryData struct {
	DataRaw    string   `json:"result,omitempty" db:"result"`
	Key        string   `json:"key,omitempty" db:"key"`
	Value      string   `json:"value,omitempty" db:"value"`
	ValueArray []string `json:"valueArray,omitempty" db:"values"`
	MapList    string   `json:"mapList,omitempty" db:"map_list"`
}

type RecordDataGet interface {
	GetId() string
	GetControlId(string) string
	GetIdList() []any
	GetSubCategory() string
	GetValue() any
}

type RecordData interface {
	GetId() string
	GetControlId(string) string
	GetIdList() []any
	SetId(string)
	SetValue(string)
	GetSubCategory() string
	GetValue() any
	GetValueMap() map[string]string
}

const (
	NO_DATA_FOUND = "NO_DATA_FOUND"
)

type RecordDataDml interface {
	SetId(string)
	CheckDuplicate(core QueryRunnerInterface) (int, string, error)
	Filler(headers map[string][]string, core QueryRunnerInterface, args ...any) (string, error)
	Post(core QueryRunnerInterface, args map[string]string) (DmlResult, int, string, error)
	CheckExistence(core QueryRunnerInterface) (int, string, error)
	PreControl(core QueryRunnerInterface) (int, string, error)
	Put(core QueryRunnerInterface, args map[string]string) (DmlResult, int, string, error)
}

func HandleCheckDuplicate(code int, desc, dupDesc string, record []QueryData, err error) (int, string, error) {
	if desc != NO_DATA_FOUND && len(record) != 0 {
		return http.StatusBadRequest, dupDesc, fmt.Errorf(dupDesc)
	}
	if desc != NO_DATA_FOUND && err != nil {
		return code, desc, err
	}
	return http.StatusOK, "", nil
}

func HandleCheckExistence(code int, desc, notExistDesc string, record []QueryData, err error) (int, string, error) {
	if err != nil {
		if desc == NO_DATA_FOUND || len(record) == 0 {
			return http.StatusBadRequest, notExistDesc, fmt.Errorf(notExistDesc)
		}
		return code, desc, err
	}
	return http.StatusOK, "", nil
}

type DmlResult struct {
	Rows         map[string]string `json:"rows" form:"rows"`
	LastInsertId int64             `json:"lastId" form:"lastId"`
	RowsAffected int64             `json:"rowsAffected" form:"rowsAffected"`
}

func (c *DmlResult) LoadFromMap(m any) error {
	data, err := json.Marshal(m.(map[string]any))
	if err == nil {
		err = json.Unmarshal(data, c)
	}
	return err
}

type FieldParser interface {
	Parse(string) string
}

func ParseCommand(command, user, app, action, title string, value map[string]string, parser FieldParser) string {
	//template := "http://{{host}}/?q={{query}}&foo={{bar}}{{bar}}"
	return fasttemplate.New(command, "{{", "}}").ExecuteString(
		map[string]any{
			"_user":             user,
			"_appName":          app,
			"_path":             action,
			"_name":             title,
			"hash3:password":    "'" + parser.Parse(value["password"]) + "'",
			"hash3:newPassword": "'" + parser.Parse(value["newPassword"]) + "'",
		},
	)
}

type QueryWithDeps interface {
	GetFillable(core QueryRunnerInterface) (map[string]any, error)
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

type Insertable interface {
	GetUniqueId() string
	GetCountCommand() string
	GetInsertCommand() (string, []any)
}

type Updatable interface {
	SetParams(args map[string]string) any
	GetUniqueId() []any
	GetCountCommand() string
	GetUpdateCommand() (string, []any)
	Finalize(QueryRunnerInterface) (string, error)
}
