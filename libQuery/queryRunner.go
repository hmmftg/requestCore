package libQuery

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/hmmftg/requestCore/libError"
)

// GetModule returns the module and program names of the query runner.
func (m QueryRunnerModel) GetModule() (string, string) {
	return m.ModuleName, m.ProgramName
}

// GetDbMode returns the database mode of the query runner.
func (m QueryRunnerModel) GetDbMode() DBMode {
	return m.Mode
}

const (
	// PrepareError is the error code for statement preparation failures.
	PrepareError = -1
	// QueryError is the error code for query execution failures.
	QueryError = -2
	// ParseError is the error code for result parsing failures.
	ParseError = -3
	// ScanError is the error code for row scanning failures.
	ScanError = -4
)

// NewStatement prepares a SQL statement on the database.
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

// GetTagValue retrieves a struct field value by its tag name and returns the field name and value.
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

// GetDBTagValue retrieves a struct field value by its "db" tag name.
func GetDBTagValue(name string, s any) (*string, *string, error) {
	return GetTagValue(name, "db", s)
}

// GetFormTagValue retrieves a struct field value by its "form" tag name.
func GetFormTagValue(name string, s any) (*string, *string, error) {
	return GetTagValue(name, "form", s)
}

const (
	// NoDataFound is the error description when a query returns no rows.
	NoDataFound = "NoDataFound"
	// NoDataFoundDesc is the human-readable description for no data found.
	NoDataFoundDesc = "رکوردی یافت نشد"
	// DuplicateFound is the error description when a query returns duplicate rows.
	DuplicateFound = "DuplicateFound"
	// DuplicateFoundDesc is the human-readable description for duplicate data.
	DuplicateFoundDesc = "رکورد تکراری است"
	// DBReadError is the error description for database read failures.
	DBReadError = "DBReadError"
	// ParseDBRespError is the error description for database response parsing failures.
	ParseDBRespError = "ParseDBRespError"
)

const (
	// QuerySingle indicates a query that expects exactly one row.
	QuerySingle QueryCommandType = iota
	// QueryAll indicates a query that returns all matching rows.
	QueryAll
	// QueryMap indicates a query that returns results as a key-value map.
	QueryMap
	// Transforms indicates a query with custom row transformation.
	Transforms
)

// QueryOld executes a query command and returns the result based on the command type.
func QueryOld[Result QueryResult](core QueryRunnerInterface, command QueryCommand, args ...any) (any, error) {
	sqlCommand := command.Command
	if len(command.CommandMap) > 0 && len(command.CommandMap[core.GetDbMode()]) > 0 {
		sqlCommand = command.CommandMap[core.GetDbMode()]
	}
	result, err := GetQuery[Result](sqlCommand, core, args...)
	if err != nil {
		if ok, err := libError.Unwrap(err); ok {
			if err.Action().Description == NoDataFound {
				return nil, errors.Join(err, libError.NewWithDescription(
					http.StatusBadRequest,
					NoDataFound,
					"no data found: %s(%s)=> %s", command.Type.String(), command.Name, NoDataFound))
			}
		}
		return nil, errors.Join(err, libError.NewWithDescription(
			http.StatusBadRequest,
			NoDataFound,
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
