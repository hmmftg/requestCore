// Package requestcore_test contains architecture validation tests that
// enforce the package dependency DAG and verify that no v2 package
// imports the v1 root module.
//
// Phase 11 removes all remaining v1 imports and dead alpha code. After
// Phase 11, no v2 package may import any v1 package
// (github.com/hmmftg/requestCore/*). The v2 module is self-contained.
//
// Run:
//
//	go test -run TestArchitecture -v ./...
package requestcore_test

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// v1ModulePath is the v1 root module path that no v2 package may import.
const v1ModulePath = "github.com/hmmftg/requestCore"

// v2ModulePath is the v2 module path.
const v2ModulePath = "github.com/hmmftg/requestCore/v2"

// stdlibOnlyPackages lists v2 packages that must import only Go
// standard library packages (no v1, no v2 sub-packages, no third-party).
// This enforces the "stdlib-only" constraint on the kernel packages.
var stdlibOnlyPackages = map[string]bool{
	"github.com/hmmftg/requestCore/v2/request":   true,
	"github.com/hmmftg/requestCore/v2/operation": true,
	"github.com/hmmftg/requestCore/v2/telemetry": true,
}

// allowedDeps specifies the allowed non-stdlib dependencies for each
// v2 package that is not stdlib-only. Packages not listed here are
// unconstrained (beyond the v1-free check).
var allowedDeps = map[string]map[string]bool{
	"github.com/hmmftg/requestCore/v2/request/faketransport": {
		"github.com/hmmftg/requestCore/v2/request": true,
	},
	"github.com/hmmftg/requestCore/v2/binding": {
		"github.com/hmmftg/requestCore/v2/request": true,
	},
	"github.com/hmmftg/requestCore/v2/validation": {
		"github.com/hmmftg/requestCore/v2/response": true,
		"github.com/go-playground/validator/v10":    true,
	},
	"github.com/hmmftg/requestCore/v2/internal/endpoint": {
		"github.com/hmmftg/requestCore/v2/request":               true,
		"github.com/hmmftg/requestCore/v2/request/faketransport": true,
		"github.com/hmmftg/requestCore/v2/operation":             true,
		"github.com/hmmftg/requestCore/v2/response":              true,
		"github.com/hmmftg/requestCore/v2/binding":               true,
		"github.com/hmmftg/requestCore/v2/validation":            true,
		"github.com/hmmftg/requestCore/v2/telemetry":             true,
		"github.com/hmmftg/requestCore/v2/renderers":             true,
	},
	"github.com/hmmftg/requestCore/v2/endpoint": {
		"github.com/hmmftg/requestCore/v2/request":               true,
		"github.com/hmmftg/requestCore/v2/request/faketransport": true,
		"github.com/hmmftg/requestCore/v2/operation":             true,
		"github.com/hmmftg/requestCore/v2/response":              true,
		"github.com/hmmftg/requestCore/v2/binding":               true,
		"github.com/hmmftg/requestCore/v2/validation":            true,
		"github.com/hmmftg/requestCore/v2/telemetry":             true,
		"github.com/hmmftg/requestCore/v2/renderers":             true,
	},
	"github.com/hmmftg/requestCore/v2/adapter": {
		"github.com/hmmftg/requestCore/v2/endpoint": true,
		"github.com/hmmftg/requestCore/v2/request":  true,
		"github.com/hmmftg/requestCore/v2/response": true,
		"github.com/hmmftg/requestCore/v2/routing":  true,
	},
	"github.com/hmmftg/requestCore/v2/internal/nextadapter": {
		"github.com/hmmftg/requestCore/v2/adapter":           true,
		"github.com/hmmftg/requestCore/v2/binding":           true,
		"github.com/hmmftg/requestCore/v2/endpoint":          true,
		"github.com/hmmftg/requestCore/v2/internal/endpoint": true,
		"github.com/hmmftg/requestCore/v2/libNetHttp":        true,
		"github.com/hmmftg/requestCore/v2/request":           true,
		"github.com/hmmftg/requestCore/v2/routing":           true,
		"github.com/hmmftg/requestCore/v2/telemetry":         true,
	},
}

// goListPackage represents the relevant fields from `go list -json`.
type goListPackage struct {
	ImportPath string
	Imports    []string
}

// TestArchitecture_V1Imports verifies that NO v2 package imports the
// v1 root module. After Phase 11, the v2 module is fully self-contained
// and must not depend on github.com/hmmftg/requestCore (v1).
func TestArchitecture_V1Imports(t *testing.T) {
	pkgs, err := listV2Packages()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	type violation struct {
		Pkg      string
		V1Import string
	}
	var violations []violation
	inventory := make(map[string][]string)

	for _, pkg := range pkgs {
		v1Imports := filterV1Imports(pkg.Imports)
		if len(v1Imports) > 0 {
			inventory[pkg.ImportPath] = v1Imports
			for _, imp := range v1Imports {
				violations = append(violations, violation{
					Pkg:      pkg.ImportPath,
					V1Import: imp,
				})
			}
		}
	}

	// Report the inventory for visibility.
	if len(inventory) > 0 {
		t.Logf("V1 import inventory (%d packages import v1):", len(inventory))
		keys := make([]string, 0, len(inventory))
		for k := range inventory {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			t.Logf("  %s -> %s", k, strings.Join(inventory[k], ", "))
		}
	} else {
		t.Logf("No v2 packages import the v1 root module.")
	}

	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("package %s imports v1: %s", v.Pkg, v.V1Import)
		}
	}
}

// listV2Packages runs `go list -json` for all v2 packages and returns
// their import paths and imports.
func listV2Packages() ([]goListPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = "."
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// go list -json outputs a stream of JSON objects, not an array.
	// Split and decode each.
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var pkgs []goListPackage
	for dec.More() {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

// filterV1Imports returns imports that reference the v1 root module,
// excluding the v2 module itself and standard library packages.
func filterV1Imports(imports []string) []string {
	var v1Imports []string
	for _, imp := range imports {
		if imp == v1ModulePath || strings.HasPrefix(imp, v1ModulePath+"/") {
			// Exclude the v2 module itself.
			if imp == v2ModulePath || strings.HasPrefix(imp, v2ModulePath+"/") {
				continue
			}
			v1Imports = append(v1Imports, imp)
		}
	}
	return v1Imports
}

// isStdlib returns true if the import path is a Go standard library
// package (no dot in the first path segment).
func isStdlib(imp string) bool {
	if imp == "" {
		return false
	}
	first := imp
	if idx := strings.Index(imp, "/"); idx >= 0 {
		first = imp[:idx]
	}
	return !strings.Contains(first, ".")
}

// TestArchitecture_DAG verifies that:
//  1. stdlib-only packages (request, operation, telemetry) import
//     only Go standard library packages.
//  2. Packages with allowedDeps only import stdlib plus their allowed
//     non-stdlib dependencies (e.g. faketransport may import request).
func TestArchitecture_DAG(t *testing.T) {
	pkgs, err := listV2Packages()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	pkgMap := make(map[string]goListPackage, len(pkgs))
	for _, p := range pkgs {
		pkgMap[p.ImportPath] = p
	}

	// Check stdlib-only packages.
	for pkgPath := range stdlibOnlyPackages {
		pkg, ok := pkgMap[pkgPath]
		if !ok {
			t.Errorf("stdlib-only package %s not found in go list output", pkgPath)
			continue
		}
		for _, imp := range pkg.Imports {
			if !isStdlib(imp) {
				t.Errorf("stdlib-only package %s imports non-stdlib: %s", pkgPath, imp)
			}
		}
	}

	// Check packages with allowed dependencies.
	for pkgPath, allowed := range allowedDeps {
		pkg, ok := pkgMap[pkgPath]
		if !ok {
			t.Errorf("package %s with allowedDeps not found in go list output", pkgPath)
			continue
		}
		for _, imp := range pkg.Imports {
			if isStdlib(imp) {
				continue
			}
			if allowed[imp] {
				continue
			}
			// Allow imports of the v2 module itself if it's the
			// package's own module (e.g. request/faketransport
			// importing v2/request is allowed via the allowedDeps
			// map).
			t.Errorf("package %s imports non-allowed dependency: %s", pkgPath, imp)
		}
	}
}

// alphaSurfacePatterns are import paths or identifiers that indicate
// removed alpha surface area. If any v2 package imports these, the
// architecture gate fails.
var alphaSurfaceImports = []string{
	"github.com/hmmftg/requestCore/v2/webFramework",
}

// alphaSurfaceIdentifiers are type/function identifiers from the
// removed alpha API. If any v2 .go file references these, the gate
// fails. This catches accidental re-introduction of alpha types.
var alphaSurfaceIdentifiers = []string{
	"HandlerRequest",
	"CallAPILogEntry",
	"webFramework.WebFramework",
	"v2wf.RequestContext",
	"v2wf.FakeParserV2",
	"v2wf.CommitState",
}

// TestArchitecture_NoAlphaSurface verifies that no v2 package imports
// the removed v2/webFramework package or references removed alpha
// surface identifiers (HandlerRequest, CallAPILogEntry, etc.).
func TestArchitecture_NoAlphaSurface(t *testing.T) {
	pkgs, err := listV2Packages()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	// Check for alpha surface imports.
	for _, pkg := range pkgs {
		for _, imp := range pkg.Imports {
			for _, alpha := range alphaSurfaceImports {
				if imp == alpha {
					t.Errorf("package %s imports removed alpha surface: %s", pkg.ImportPath, imp)
				}
			}
		}
	}

	// Check for alpha surface identifiers in .go files.
	// Use grep to search all .go files (excluding this test file).
	cmd := exec.Command("grep", "-rn", "--include=*.go",
		"-e", strings.Join(alphaSurfaceIdentifiers, " -e "),
		".")
	cmd.Dir = "."
	// Exclude this test file from the search.
	cmd.Args = append(cmd.Args[:len(cmd.Args)-1], "--exclude=archcheck_test.go", ".")
	out, _ := cmd.CombinedOutput()
	if len(out) > 0 {
		t.Errorf("alpha surface identifiers found in v2 source files:\n%s", string(out))
	}
}
