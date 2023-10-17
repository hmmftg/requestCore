package testingtools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/libQuery"
)

const (
	insertCommand      string = "insert"
	updateCommand      string = "update"
	deleteCommand      string = "delete"
	selectCommand      string = "select"
	dmlCommands        string = "DmlCommands"
	preControlCommands string = "PreControlCommands"
	finalizeCommands   string = "FinalizeCommands"
)

// TestDMLs tests a model  for equality of commands arguments, and given argumants,
// also it checks if given type is the same is the command.
func TestDMLs[Model any, PT interface {
	libQuery.DmlModel
	*Model
}](t *testing.T) {
	dml := PT(new(Model))

	parseMethod(t, dml.DmlCommands(), dmlCommands)
	parseMethod(t, dml.PreControlCommands(), preControlCommands)
	parseMethod(t, dml.FinalizeCommands(), finalizeCommands)
}

// parseMethod parses a single dml method.
func parseMethod(t *testing.T, cmds map[string][]libQuery.DmlCommand, methodName string) {
	t.Run(methodName, func(t *testing.T) {
		t.Parallel()

		for logicName, cmd := range cmds {
			// Variable shadowing in this section is because of the t.Parallel and how Go's goroutines works,
			// this is a unique case that shadowing is allowed.
			// The bug is that in a Go for loop, the loop variable is reused for each iteration,
			// so the logicName and cmd variables are shared across all goroutines. That's not what we want,
			// each goroutine should have it's own specific variable.
			//
			// For more information see the below articles:
			// 1- Be Careful with Table Driven Tests and t.Parallel():
			// 		https://gist.github.com/posener/92a55c4cd441fc5e5e85f27bca008721
			// 2-(Go common mistakes guide) Using goroutines on loop iterator variables:
			//		https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
			logicName := logicName
			cmd := cmd

			t.Run(logicName, func(t *testing.T) {
				t.Parallel()

				for cmdIndex, cmdValue := range cmd {
					parseDML(t, cmdValue, fmt.Sprintf("%s:%s:%d:%s", methodName, logicName, cmdIndex, cmdValue.Name))
				}
			})
		}
	})
}

// parseDML parses a command for checking it's arguments and type.
func parseDML(t *testing.T, cmd libQuery.DmlCommand, path string) {
	commandArgsLen := strings.Count(cmd.Command, pgSign) + strings.Count(cmd.Command, oracleSign)

	checkNumberOfArgs(t, path, commandArgsLen, len(cmd.Args))

	checkType(t, cmd, path)
}

// checkNumberOfArgs checks if command arguments are equal to the given arguments.
func checkNumberOfArgs(t *testing.T, path string, commandArgsLen, argsLen int) {
	if commandArgsLen != argsLen {
		t.Fatalf("error at (%s) command has %d argument, But args has %d argument", path, commandArgsLen, argsLen)
	}
}

// checkType checks if the given type is as the same as the command.
func checkType(t *testing.T, cmd libQuery.DmlCommand, path string) {
	commnd := strings.ToLower(cmd.Command)
	var isSame bool

	switch cmd.Type {
	case libQuery.Insert:
		isSame = strings.Contains(commnd, insertCommand)
	case libQuery.Update:
		isSame = strings.Contains(commnd, updateCommand)
	case libQuery.Delete:
		isSame = strings.Contains(commnd, deleteCommand)
	case libQuery.QueryCheckExists:
		isSame = strings.Contains(commnd, selectCommand)
	case libQuery.QueryCheckNotExists:
		isSame = strings.Contains(commnd, selectCommand)
	default:
		isSame = false
	}

	if !isSame {
		t.Fatalf("error at (%s) type in command is not the same as the given type: %s", path, cmd.Type)
	}
}
