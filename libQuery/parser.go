package libQuery

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
	"github.com/valyala/fasttemplate"
)

func ParseQueryResult(result map[string]any, t reflect.Type, v reflect.Value) {
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		switch value := result[tag].(type) {
		case string:
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
				} else if t.Field(i).Type.Kind() == reflect.Ptr {
					if t.Field(i).Type.String() == "*time.Time" {
						if len(value) > 0 {
							format := t.Field(i).Tag.Get("timeFormat")
							tm, errParseTime := time.Parse(t.Field(i).Tag.Get("timeFormat"), value)
							if errParseTime != nil {
								log.Println("unable to parse time field with tag: timeFormat=", format, errParseTime)
							} else {
								v.FieldByName(t.Field(i).Name).Set(reflect.ValueOf(&tm))
							}
						}
					} else {
						log.Println(
							"no parser defined for name:", t.Field(i).Type.Name(),
							"path:", t.Field(i).Type.PkgPath(),
							"string:", t.Field(i).Type.String())
					}
				} else {
					v.FieldByName(t.Field(i).Name).SetString(value)
				}
			}
		case bool:
			v.FieldByName(t.Field(i).Name).SetBool(value)
		case int64:
			switch t.Field(i).Type.Kind() {
			case reflect.Int64:
				v.FieldByName(t.Field(i).Name).SetInt(value)
			case reflect.Float64:
				v.FieldByName(t.Field(i).Name).SetFloat(float64(value))
			default:
				log.Printf("ParseQueryResult, unknown int64 sub-type: %s->%T\n",
					v.FieldByName(t.Field(i).Name).Type().String(),
					result[tag])
			}
		case float64:
			switch t.Field(i).Type.Kind() {
			case reflect.Int64:
				v.FieldByName(t.Field(i).Name).SetInt(int64(value))
			case reflect.Float64:
				v.FieldByName(t.Field(i).Name).SetFloat(value)
			default:
				log.Printf("ParseQueryResult, unknown float sub-type: %s->%T\n",
					v.FieldByName(t.Field(i).Name).Type().String(),
					result[tag])
			}
		case time.Time:
			v.FieldByName(t.Field(i).Name).Set(reflect.ValueOf(result[tag]))
		case nil:
		case []uint8:
			switch t.Field(i).Type.Kind() {
			case reflect.String:
				v.FieldByName(t.Field(i).Name).SetString(string(value))
			case reflect.Slice:
				sValue := string(value)
				if strings.HasPrefix(sValue, "{") && strings.HasSuffix(sValue, "}") {
					values := strings.Split(sValue[1:len(sValue)-1], ",")
					slice := reflect.MakeSlice(reflect.TypeOf([]string{}), 0, 0)
					for _, member := range values {
						newSlice := reflect.Append(slice, reflect.ValueOf(member))
						v.FieldByName(t.Field(i).Name).Set(newSlice)
						slice = v.FieldByName(t.Field(i).Name)
					}
				}
			default:
				log.Printf("ParseQueryResult, unknown []uint8 sub-type: %s->%T\n",
					v.FieldByName(t.Field(i).Name).Type().String(),
					value)
			}
		default:
			log.Printf("ParseQueryResult, unknown type: %T\n", result[tag])
		}

	}
}

func ConvertJsonToStruct[Q any](row string) (Q, error) {
	var rowObj Q
	err := json.Unmarshal([]byte(row), &rowObj)
	if err != nil {
		return rowObj, err
	}

	return rowObj, nil
}

func (c *DmlResult) LoadFromMap(m any) error {
	data, err := json.Marshal(m.(map[string]any))
	if err == nil {
		err = json.Unmarshal(data, c)
	}
	return err
}

func ParseCommand(command, user, app, action, title string, value map[string]string, parser webFramework.FieldParser) string {
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

func SerializeStringArray(arr []string) string {
	var result strings.Builder
	result.WriteRune('{')
	result.WriteString(strings.Join(arr, ","))
	result.WriteRune('}')
	return result.String()
}

func SerializeArray(arr []any) string {
	result := "{"
	for _, member := range arr {
		result += fmt.Sprintf("%v,", member)
	}
	return result[:len(result)-1] + "}"
}

func PrepareArgs(args []any) []any {
	preparedArgs := args
	for id := range args {
		switch arg := args[id].(type) {
		case []string:
			preparedArgs[id] = SerializeStringArray(arg)
		case []int64:
			preparedArgs[id] = SerializeArray(args)
		case []any:
			preparedArgs[id] = SerializeArray(args)
		}
	}
	return preparedArgs
}
