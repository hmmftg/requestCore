// Code generated by "enumer -type=QueryCommandType -json -output queryEnum.go"; DO NOT EDIT.

package libQuery

import (
	"encoding/json"
	"fmt"
	"strings"
)

const _QueryCommandTypeName = "QuerySingleQueryAllQueryMapTransforms"

var _QueryCommandTypeIndex = [...]uint8{0, 11, 19, 27, 37}

const _QueryCommandTypeLowerName = "querysinglequeryallquerymaptransforms"

func (i QueryCommandType) String() string {
	if i < 0 || i >= QueryCommandType(len(_QueryCommandTypeIndex)-1) {
		return fmt.Sprintf("QueryCommandType(%d)", i)
	}
	return _QueryCommandTypeName[_QueryCommandTypeIndex[i]:_QueryCommandTypeIndex[i+1]]
}

// An "invalid array index" compiler error signifies that the constant values have changed.
// Re-run the stringer command to generate them again.
func _QueryCommandTypeNoOp() {
	var x [1]struct{}
	_ = x[QuerySingle-(0)]
	_ = x[QueryAll-(1)]
	_ = x[QueryMap-(2)]
	_ = x[Transforms-(3)]
}

var _QueryCommandTypeValues = []QueryCommandType{QuerySingle, QueryAll, QueryMap, Transforms}

var _QueryCommandTypeNameToValueMap = map[string]QueryCommandType{
	_QueryCommandTypeName[0:11]:       QuerySingle,
	_QueryCommandTypeLowerName[0:11]:  QuerySingle,
	_QueryCommandTypeName[11:19]:      QueryAll,
	_QueryCommandTypeLowerName[11:19]: QueryAll,
	_QueryCommandTypeName[19:27]:      QueryMap,
	_QueryCommandTypeLowerName[19:27]: QueryMap,
	_QueryCommandTypeName[27:37]:      Transforms,
	_QueryCommandTypeLowerName[27:37]: Transforms,
}

var _QueryCommandTypeNames = []string{
	_QueryCommandTypeName[0:11],
	_QueryCommandTypeName[11:19],
	_QueryCommandTypeName[19:27],
	_QueryCommandTypeName[27:37],
}

// QueryCommandTypeString retrieves an enum value from the enum constants string name.
// Throws an error if the param is not part of the enum.
func QueryCommandTypeString(s string) (QueryCommandType, error) {
	if val, ok := _QueryCommandTypeNameToValueMap[s]; ok {
		return val, nil
	}

	if val, ok := _QueryCommandTypeNameToValueMap[strings.ToLower(s)]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%s does not belong to QueryCommandType values", s)
}

// QueryCommandTypeValues returns all values of the enum
func QueryCommandTypeValues() []QueryCommandType {
	return _QueryCommandTypeValues
}

// QueryCommandTypeStrings returns a slice of all String values of the enum
func QueryCommandTypeStrings() []string {
	strs := make([]string, len(_QueryCommandTypeNames))
	copy(strs, _QueryCommandTypeNames)
	return strs
}

// IsAQueryCommandType returns "true" if the value is listed in the enum definition. "false" otherwise
func (i QueryCommandType) IsAQueryCommandType() bool {
	for _, v := range _QueryCommandTypeValues {
		if i == v {
			return true
		}
	}
	return false
}

// MarshalJSON implements the json.Marshaler interface for QueryCommandType
func (i QueryCommandType) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface for QueryCommandType
func (i *QueryCommandType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("QueryCommandType should be a string, got %s", data)
	}

	var err error
	*i, err = QueryCommandTypeString(s)
	return err
}
