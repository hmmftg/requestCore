package libQuery

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

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
