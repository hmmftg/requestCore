package testingtools

import (
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/libQuery"
)

func TestFetchData[Model any, PT interface {
	libQuery.QueryRequest
	*Model
}](t *testing.T, queryList map[string]libQuery.QueryCommand) {
	m := PT(new(Model))
	for key, command := range queryList {
		parseQuery(t, command, key, m.QueryArgs()[key])
	}
}

func parseQuery(t *testing.T, cmd libQuery.QueryCommand, key string, args []any) {
	cmdcount := strings.Count(cmd.Command, oracleSign) + strings.Count(cmd.Command, pgSign)
	if cmdcount != len(args) {
		t.Fatal("Count of query args is not equal to parameter", cmdcount, cmd.Name, key)
	}
}
