package libQuery

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/hmmftg/requestCore/webFramework"
	"github.com/valyala/fasttemplate"
)

func parseTimeUsingTimeFormat(field reflect.StructField, value string) (reflect.Value, error) {
	if len(value) > 0 {
		format := field.Tag.Get("timeFormat")
		tm, errParseTime := time.Parse(field.Tag.Get("timeFormat"), value)
		if errParseTime != nil {
			return reflect.Value{}, fmt.Errorf("unable to parse time field with timeFormat tag: %s, => %s", format, errParseTime.Error())
		} else {
			return reflect.ValueOf(&tm), nil
		}
	}
	return reflect.Value{}, errors.New("empty value")
}

func ParseQueryResult(result map[string]any, t reflect.Type, v reflect.Value) {
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("db")
		switch value := result[tag].(type) {
		case string:
			if value == "true" || value == "false" {
				switch t.Field(i).Type.String() {
				case "bool":
					bl, _ := strconv.ParseBool(value)
					v.FieldByName(t.Field(i).Name).SetBool(bl)
				case "string":
					v.FieldByName(t.Field(i).Name).SetString(value)
				default:
					slog.Error("ParseQueryResult, unknown bool",
						slog.String("tag", tag),
						slog.String("desired", t.Field(i).Type.String()),
						slog.String("result", fmt.Sprintf("%T", result[tag])),
					)
				}
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
						newV, err := parseTimeUsingTimeFormat(t.Field(i), value)
						if err != nil {
							slog.Error("error in parseTimeUsingTimeFormat",
								slog.String("tag", tag),
								slog.Any("error", err),
								slog.String("data", value),
							)
							continue
						}
						v.FieldByName(t.Field(i).Name).Set(newV)
					} else {
						slog.Error("no parser defined for name",
							slog.String("tag", tag),
							slog.String("path", t.Field(i).Type.PkgPath()),
							slog.String("name", t.Field(i).Type.Name()),
							slog.String("data", t.Field(i).Type.String()),
						)
					}
				} else {
					switch t.Field(i).Type.String() {
					case "bool": // value is not true or false(first if condition), it should be 0 or 1 then
						v.FieldByName(t.Field(i).Name).SetBool(value == "1")
					case "string":
						v.FieldByName(t.Field(i).Name).SetString(value)
					default:
						slog.Error("ParseQueryResult, unknown string",
							slog.String("tag", tag),
							slog.String("desired", t.Field(i).Type.String()),
							slog.String("result", fmt.Sprintf("%T", result[tag])),
						)
					}
				}
			}
		case bool:
			switch t.Field(i).Type.String() {
			case "bool":
				v.FieldByName(t.Field(i).Name).SetBool(value)
			case "string":
				if value {
					v.FieldByName(t.Field(i).Name).SetString("true")
				} else {
					v.FieldByName(t.Field(i).Name).SetString("false")
				}
			default:
				slog.Error("ParseQueryResult, unknown bool",
					slog.String("tag", tag),
					slog.String("desired", t.Field(i).Type.String()),
					slog.String("result", fmt.Sprintf("%T", result[tag])),
				)
			}
		case int64:
			switch t.Field(i).Type.Kind() {
			case reflect.Int64:
				v.FieldByName(t.Field(i).Name).SetInt(value)
			case reflect.Float64:
				v.FieldByName(t.Field(i).Name).SetFloat(float64(value))
			default:
				slog.Error("ParseQueryResult, unknown int64 sub-type",
					slog.String("tag", tag),
					slog.String("desired", v.FieldByName(t.Field(i).Name).Type().String()),
					slog.String("result", fmt.Sprintf("%T", result[tag])),
				)
			}
		case uint64:
			switch t.Field(i).Type.Kind() {
			case reflect.Int64:
				v.FieldByName(t.Field(i).Name).SetInt(int64(value))
			case reflect.Uint64:
				v.FieldByName(t.Field(i).Name).SetUint(value)
			default:
				slog.Error("ParseQueryResult, unknown int64 sub-type",
					slog.String("tag", tag),
					slog.String("desired", v.FieldByName(t.Field(i).Name).Type().String()),
					slog.String("result", fmt.Sprintf("%T", result[tag])),
				)
			}
		case float64:
			switch t.Field(i).Type.Kind() {
			case reflect.Int64:
				v.FieldByName(t.Field(i).Name).SetInt(int64(value))
			case reflect.Float64:
				v.FieldByName(t.Field(i).Name).SetFloat(value)
			default:
				slog.Error("ParseQueryResult, unknown float sub-type",
					slog.String("tag", tag),
					slog.String("desired", v.FieldByName(t.Field(i).Name).Type().String()),
					slog.String("result", fmt.Sprintf("%T", result[tag])),
				)
			}
		case *time.Time:
			switch t.Field(i).Type.String() {
			case "*time.Time":
				v.FieldByName(t.Field(i).Name).Set(reflect.ValueOf(result[tag]))
			default:
				slog.Error("ParseQueryResult, unknown *time.Time sub-type",
					slog.String("tag", tag),
					slog.String("desired", t.Field(i).Type.String()),
					slog.String("result", fmt.Sprintf("%T", result[tag])),
				)
			}
		case time.Time:
			switch t.Field(i).Type.String() {
			case "time.Time":
				v.FieldByName(t.Field(i).Name).Set(reflect.ValueOf(result[tag]))
			default:
				slog.Error("ParseQueryResult, unknown time.Time sub-type",
					slog.String("tag", tag),
					slog.String("desired", t.Field(i).Type.String()),
					slog.String("result", fmt.Sprintf("%T", result[tag])),
				)
			}
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
				slog.Error("ParseQueryResult, unknown []uint8 sub-type",
					slog.String("tag", tag),
					slog.String("desired", v.FieldByName(t.Field(i).Name).Type().String()),
					slog.String("result", fmt.Sprintf("%T", value)),
				)
			}
		default:
			slog.Error("ParseQueryResult, unknown type",
				slog.String("tag", tag),
				slog.String("result", fmt.Sprintf("%T", result[tag])),
			)
		}

	}
}

func ParseMap[Target any](input map[string]any) (*Target, error) {
	result := new(Target)
	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Metadata:         nil,
		Result:           result,
		TagName:          "db",
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(input)
	return result, err
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
