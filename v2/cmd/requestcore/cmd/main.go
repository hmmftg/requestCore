package main

import (
	"fmt"
	"os"

	"github.com/hmmftg/requestCore/v2/cmd/requestcore"
)

func main() {
	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
