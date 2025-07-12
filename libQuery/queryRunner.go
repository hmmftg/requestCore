package libQuery

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
)

func (m QueryRunnerModel) GetModule() (string, string) {
	return m.ModuleName, m.ProgramName
}

func (m QueryRunnerModel) GetDbMode() DBMode {
	return m.Mode
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
		slog.Error("error in ping", slog.Any("error", errPing))
	}
	stmt, err := m.DB.Prepare(command)
	if err != nil {
		return nil, libError.Join(err, "QueryRunner[prepare](%s)", command)
	}
	return stmt, nil
}

func QueryToStruct[Target any](q QueryRunnerInterface, querySql string, args ...any) ([]Target, error) {
	stmt, err := q.NewStatement(querySql)
	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_INITIALIZE_STATEMENT",
				"queryRunner[prepare](%s,%v)", querySql, args,
			))
	}
	defer stmt.Close()
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_QUERY_STATEMENT",
				"queryRunner[query](%s,%v)", querySql, args,
			))
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, errors.Join(err,
			libError.NewWithDescription(
				status.InternalServerError,
				"UNABLE_TO_GET_COLUMN_TYPES",
				"queryRunner[ColumnTypes](%s,%v)", querySql, args,
			))
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
			return nil, errors.Join(err,
				libError.NewWithDescription(
					status.InternalServerError,
					"UNABLE_TO_GET_SCAN_ROW",
					"queryRunner[Scan](%s,%v)", querySql, scanArgs,
				))
		}

		masterData := map[string]any{}

		for i, v := range columnTypes {
			masterData[v.Name()] = scanArgs[i].(*sql.Null[any]).V
		}

		parsed, err := ParseMap[Target](masterData)
		if parsed == nil {
			return nil, errors.Join(err,
				libError.NewWithDescription(
					status.InternalServerError,
					"UNABLE_TO_GET_SCAN_ROW",
					"queryRunner[parse](%s,%v)", querySql, masterData,
				))
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

const (
	NO_DATA_FOUND        = "NO_DATA_FOUND"
	NO_DATA_FOUND_DESC   = "رکوردی یافت نشد"
	DUPLICATE_FOUND      = "DUPLICATE_FOUND"
	DUPLICATE_FOUND_DESC = "رکورد تکراری است"
	DB_READ_ERROR        = "DB_READ_ERROR"
	PARSE_DB_RESP_ERROR  = "PARSE_DB_RESP_ERROR"
)

func GetQuery[R any](query string, core QueryRunnerInterface, args ...any) ([]R, error) {
	//Query
	rows, err := QueryToStruct[R](core, query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, DB_READ_ERROR, "unable to execute query"),
		)
	}
	if len(rows) == 0 {
		return nil, libError.NewWithDescription(
			http.StatusBadRequest,
			NO_DATA_FOUND,
			"no data found: %s,%v", query, args,
		)
	}
	return rows, nil
}

type CommandInterface interface {
	GetCommand(DBMode) string
	GetArgs() []any
	GetType() int
}

func Query[R any](command CommandInterface, core QueryRunnerInterface, args ...any) ([]R, error) {
	if command.GetType() == int(QueryMap) {
		return nil, libError.NewWithDescription(status.BadRequest, DB_READ_ERROR, "unsupported command type")
	}
	query := command.GetCommand(core.GetDbMode())
	//Query
	rows, err := QueryToStruct[R](core, query, args...)
	if err != nil {
		return nil, errors.Join(
			err,
			libError.NewWithDescription(status.InternalServerError, DB_READ_ERROR, "unable to execute query"),
		)
	}
	switch command.GetType() {
	case int(QuerySingle):
		if len(rows) == 0 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				NO_DATA_FOUND,
				"no data found: %s,%v", query, args,
			)
		}
		if len(rows) > 1 {
			return nil, libError.NewWithDescription(
				http.StatusBadRequest,
				DUPLICATE_FOUND,
				"duplicate data found: %s,%v,%v", query, args, rows,
			)
		}
		return rows, nil
	case int(QueryAll):
		return rows, nil
	}
	return nil, nil
}

const (
	QuerySingle QueryCommandType = iota
	QueryAll
	QueryMap
	Transforms
)

func QueryOld[Result QueryResult](core QueryRunnerInterface, command QueryCommand, args ...any) (any, error) {
	sqlCommand := command.Command
	if len(command.CommandMap) > 0 && len(command.CommandMap[core.GetDbMode()]) > 0 {
		sqlCommand = command.CommandMap[core.GetDbMode()]
	}
	result, err := GetQuery[Result](sqlCommand, core, args...)
	if err != nil {
		if ok, err := libError.Unwrap(err); ok {
			if err.Action().Description == NO_DATA_FOUND {
				return nil, errors.Join(err, libError.NewWithDescription(
					http.StatusBadRequest,
					NO_DATA_FOUND,
					"no data found: %s(%s)=> %s", command.Type.String(), command.Name, NO_DATA_FOUND))
			}
		}
		return nil, errors.Join(err, libError.NewWithDescription(
			http.StatusBadRequest,
			NO_DATA_FOUND,
			"error call sql: %s(%s)", command.Type.String(), command.Name))
	}
	switch command.Type {
	case QuerySingle:
		return result[0], nil
	case QueryAll:
		return result, nil
	case QueryMap:
		respMap := make(map[string]any)
		for _, record := range result {
			respMap[record.GetID()] = record.GetValue()
		}
		return respMap, nil
	}
	return nil, nil
}
