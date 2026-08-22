// Package cmd provides the requestcore CLI for generating v2 application
// scaffolding, including handlers, resources, middleware, and project structure.
//
// Usage:
//
//	requestcore generate handler <name>
//	requestcore generate resource <name>
//	requestcore generate middleware <name>
//	requestcore generate project <name>
//	requestcore version
package cmd

import (
	"fmt"
	"os"
	"strings"
)

// Command represents a CLI command.
type Command struct {
	Name        string
	Description string
	Run         func(args []string) error
}

// Commands returns the available CLI commands.
func Commands() []Command {
	return []Command{
		{
			Name:        "version",
			Description: "Print the requestcore v2 version",
			Run:         runVersion,
		},
		{
			Name:        "generate handler",
			Description: "Generate a new v2 handler file",
			Run:         runGenerateHandler,
		},
		{
			Name:        "generate resource",
			Description: "Generate a new v2 resource file with 7 operations",
			Run:         runGenerateResource,
		},
		{
			Name:        "generate middleware",
			Description: "Generate a new v2 middleware file",
			Run:         runGenerateMiddleware,
		},
		{
			Name:        "generate project",
			Description: "Generate a new v2 project structure",
			Run:         runGenerateProject,
		},
	}
}

// Execute runs the CLI with the given arguments.
func Execute(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmdName := args[0]
	if cmdName == "generate" && len(args) > 1 {
		cmdName = "generate " + args[1]
		args = args[1:]
	}

	for _, cmd := range Commands() {
		if cmd.Name == cmdName {
			return cmd.Run(args[1:])
		}
	}

	return fmt.Errorf("unknown command: %s", cmdName)
}

func printUsage() {
	fmt.Println("requestcore v2 CLI")
	fmt.Println()
	fmt.Println("Commands:")
	for _, cmd := range Commands() {
		fmt.Printf("  %-25s %s\n", cmd.Name, cmd.Description)
	}
}

// Version is the requestcore CLI version. It can be overridden at build
// time via -ldflags "-X github.com/hmmftg/requestCore/v2/cmd/requestcore.Version=v2.0.0".
// When not overridden, it defaults to "v2.0.0-alpha".
var Version = "v2.0.0-alpha"

func runVersion(args []string) error {
	fmt.Printf("requestcore %s\n", Version)
	return nil
}

func runGenerateHandler(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: requestcore generate handler <name>")
	}
	name := args[0]
	return generateHandler(name)
}

func runGenerateResource(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: requestcore generate resource <name>")
	}
	name := args[0]
	return generateResource(name)
}

func runGenerateMiddleware(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: requestcore generate middleware <name>")
	}
	name := args[0]
	return generateMiddleware(name)
}

func runGenerateProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: requestcore generate project <name>")
	}
	name := args[0]
	return generateProject(name)
}

// toPascalCase converts a kebab-case or snake_case name to PascalCase.
func toPascalCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

// toCamelCase converts a kebab-case or snake_case name to camelCase.
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) > 0 {
		return strings.ToLower(pascal[:1]) + pascal[1:]
	}
	return pascal
}

// writeFile writes content to a file, creating parent directories if needed.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
