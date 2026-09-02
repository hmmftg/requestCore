// Package requestcore_test contains architecture validation tests that
// enforce the package dependency DAG and track v1 root module imports.
//
// In Tranche 0, this test ran in inventory mode. Starting in Tranche 1,
// newly introduced canonical kernel packages (request, operation,
// telemetry, request/faketransport) are added to v1FreePackages and
// forbidden from importing v1. Later tranches add more packages as
// they are introduced.
//
// Run:
//
//	go test -run TestArchitecture -v ./...
//
// The V1Imports test fails if a package in v1FreePackages imports v1.
// The DAG test verifies that kernel packages only import allowed
// dependencies.
package requestcore_test

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// v1ModulePath is the v1 root module path that canonical v2 packages
// must not import. Only compat/* packages are allowed to import v1.
const v1ModulePath = "github.com/hmmftg/requestCore"

// v2ModulePath is the v2 module path.
const v2ModulePath = "github.com/hmmftg/requestCore/v2"

// v1FreePackages lists v2 packages that must NOT import the v1 root
// module. Tranche 1 adds the new kernel packages here. Later tranches
// add more canonical packages as they are introduced.
//
// internal/nextadapter is intentionally NOT in this list: it is the
// narrow bridge package permitted to import root webFramework for
// AddLog forwarding. See v1BridgeAllowlist.
var v1FreePackages = map[string]bool{
	"github.com/hmmftg/requestCore/v2/request":               true,
	"github.com/hmmftg/requestCore/v2/request/faketransport": true,
	"github.com/hmmftg/requestCore/v2/operation":             true,
	"github.com/hmmftg/requestCore/v2/telemetry":             true,
	"github.com/hmmftg/requestCore/v2/binding":               true,
	"github.com/hmmftg/requestCore/v2/validation":            true,
	"github.com/hmmftg/requestCore/v2/internal/endpoint":     true,
	// Tranche 4 will add: adapter/*, routing, handlers, app
}

// v1BridgeAllowlist lists v2 packages permitted to import v1 root
// packages, along with the exact set of v1 imports they may use.
// internal/nextadapter is the only bridge package and may import only
// root webFramework for AddLog forwarding.
var v1BridgeAllowlist = map[string]map[string]bool{
	"github.com/hmmftg/requestCore/v2/internal/nextadapter": {
		"github.com/hmmftg/requestCore/webFramework": true,
	},
}

// knownV1Importers lists pre-existing v2-alpha packages that already
// imported v1 before Tranche 3. These are documented but not yet
// constrained; Tranche 4 will migrate them to the new kernel and
// remove them from this list. New packages must NOT be added here —
// they must use v1BridgeAllowlist or remain v1-free.
var knownV1Importers = map[string]bool{
	"github.com/hmmftg/requestCore/v2":                 true,
	"github.com/hmmftg/requestCore/v2/app":             true,
	"github.com/hmmftg/requestCore/v2/examples/simple": true,
	"github.com/hmmftg/requestCore/v2/handlers":        true,
	"github.com/hmmftg/requestCore/v2/libChi":          true,
	"github.com/hmmftg/requestCore/v2/libFiber":        true,
	"github.com/hmmftg/requestCore/v2/libGin":          true,
	"github.com/hmmftg/requestCore/v2/libNetHttp":      true,
	"github.com/hmmftg/requestCore/v2/resources":       true,
	"github.com/hmmftg/requestCore/v2/response":        true,
	"github.com/hmmftg/requestCore/v2/session":         true,
	"github.com/hmmftg/requestCore/v2/testingtools":    true,
	"github.com/hmmftg/requestCore/v2/webFramework":    true,
	"github.com/hmmftg/requestCore/v2/workers":         true,
}

// stdlibOnlyPackages lists v2 packages that must import only Go
// standard library packages (no v1, no v2 sub-packages, no third-party).
// This enforces the "stdlib-only" constraint on the request package.
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
		"github.com/hmmftg/requestCore/v2/webFramework":      true,
		"github.com/hmmftg/requestCore/webFramework":         true, // single bridge exception
	},
}

// goListPackage represents the relevant fields from `go list -json`.
type goListPackage struct {
	ImportPath string
	Imports    []string
}

// TestArchitecture_V1Imports inventories which v2 packages import the
// v1 root module and enforces that:
//  1. Packages in v1FreePackages do not import v1.
//  2. Bridge packages in v1BridgeAllowlist only import their allowed
//     v1 packages.
//  3. No package outside v1FreePackages and v1BridgeAllowlist imports
//     v1 (new v1 imports must be explicitly allowed).
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
	var bridgeViolations []violation
	var unlistedViolations []violation
	inventory := make(map[string][]string)

	for _, pkg := range pkgs {
		v1Imports := filterV1Imports(pkg.Imports)
		if len(v1Imports) > 0 {
			inventory[pkg.ImportPath] = v1Imports
			if v1FreePackages[pkg.ImportPath] {
				for _, imp := range v1Imports {
					violations = append(violations, violation{
						Pkg:      pkg.ImportPath,
						V1Import: imp,
					})
				}
			} else if allowed, isBridge := v1BridgeAllowlist[pkg.ImportPath]; isBridge {
				for _, imp := range v1Imports {
					if !allowed[imp] {
						bridgeViolations = append(bridgeViolations, violation{
							Pkg:      pkg.ImportPath,
							V1Import: imp,
						})
					}
				}
			} else if !knownV1Importers[pkg.ImportPath] {
				for _, imp := range v1Imports {
					unlistedViolations = append(unlistedViolations, violation{
						Pkg:      pkg.ImportPath,
						V1Import: imp,
					})
				}
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
			t.Errorf("package %s is in v1FreePackages but imports v1: %s", v.Pkg, v.V1Import)
		}
	}
	if len(bridgeViolations) > 0 {
		for _, v := range bridgeViolations {
			t.Errorf("package %s is a bridge but imports a non-allowed v1 package: %s", v.Pkg, v.V1Import)
		}
	}
	if len(unlistedViolations) > 0 {
		for _, v := range unlistedViolations {
			t.Errorf("package %s imports v1 but is not in v1FreePackages or v1BridgeAllowlist: %s", v.Pkg, v.V1Import)
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
