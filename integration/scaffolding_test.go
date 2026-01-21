package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestScaffoldingRequiresModuleFlag tests that juango init requires -m flag
func TestScaffoldingRequiresModuleFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "juango-scaffold-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build juango CLI
	juangoBin := filepath.Join(tmpDir, "juango")
	buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
	buildCmd.Dir = projectRoot()
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build juango: %v\n%s", err, output)
	}

	// Try init without -m flag - should fail
	initCmd := exec.Command(juangoBin, "init", "myapp")
	initCmd.Dir = tmpDir
	output, err := initCmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected init to fail without -m flag, but it succeeded")
	}

	if !strings.Contains(string(output), "required flag") || !strings.Contains(string(output), "module") {
		t.Errorf("Expected error about required module flag, got: %s", output)
	}
}

// TestScaffoldingRequiresNpm tests that juango init checks for npm
func TestScaffoldingRequiresNpm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "juango-scaffold-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build juango CLI
	juangoBin := filepath.Join(tmpDir, "juango")
	buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
	buildCmd.Dir = projectRoot()
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build juango: %v\n%s", err, output)
	}

	// Run init with empty PATH (no npm available)
	initCmd := exec.Command(juangoBin, "init", "myapp", "-m", "github.com/test/myapp")
	initCmd.Dir = tmpDir
	initCmd.Env = []string{"PATH="} // Empty PATH

	output, err := initCmd.CombinedOutput()

	if err == nil {
		t.Fatal("Expected init to fail without npm in PATH, but it succeeded")
	}

	if !strings.Contains(string(output), "npm not found") {
		t.Errorf("Expected error about npm not found, got: %s", output)
	}
}

// TestScaffoldingModulePath tests that the provided module path is used correctly
func TestScaffoldingModulePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if npm not available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not available, skipping test")
	}

	tests := []struct {
		name           string
		projectName    string
		moduleFlag     string
		expectedModule string
	}{
		{
			name:           "github module path",
			projectName:    "myapp",
			moduleFlag:     "github.com/myorg/myapp",
			expectedModule: "github.com/myorg/myapp",
		},
		{
			name:           "gitlab module path",
			projectName:    "myapp",
			moduleFlag:     "gitlab.com/myuser/myapp",
			expectedModule: "gitlab.com/myuser/myapp",
		},
		{
			name:           "custom domain",
			projectName:    "myapp",
			moduleFlag:     "git.example.com/team/myapp",
			expectedModule: "git.example.com/team/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "juango-scaffold-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Build juango CLI
			juangoBin := filepath.Join(tmpDir, "juango")
			buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
			buildCmd.Dir = projectRoot()
			if output, err := buildCmd.CombinedOutput(); err != nil {
				t.Fatalf("Failed to build juango: %v\n%s", err, output)
			}

			// Run init with module flag
			initCmd := exec.Command(juangoBin, "init", tt.projectName, "-m", tt.moduleFlag)
			initCmd.Dir = tmpDir
			if output, err := initCmd.CombinedOutput(); err != nil {
				t.Fatalf("juango init failed: %v\n%s", err, output)
			}

			// Check the generated go.mod
			goModPath := filepath.Join(tmpDir, tt.projectName, "go.mod")
			goModContent, err := os.ReadFile(goModPath)
			if err != nil {
				t.Fatalf("Failed to read go.mod: %v", err)
			}

			// Extract module path from go.mod
			moduleRegex := regexp.MustCompile(`module\s+(\S+)`)
			matches := moduleRegex.FindStringSubmatch(string(goModContent))
			if len(matches) < 2 {
				t.Fatalf("Could not find module path in go.mod:\n%s", goModContent)
			}

			actualModule := matches[1]
			if actualModule != tt.expectedModule {
				t.Errorf("Module path mismatch:\n  expected: %s\n  actual:   %s", tt.expectedModule, actualModule)
			}
		})
	}
}

// TestScaffoldingFrontendDependencies tests that the frontend has no invalid npm dependencies
func TestScaffoldingFrontendDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if npm not available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "juango-scaffold-frontend-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build and run juango init
	juangoBin := filepath.Join(tmpDir, "juango")
	buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
	buildCmd.Dir = projectRoot()
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build juango: %v\n%s", err, output)
	}

	initCmd := exec.Command(juangoBin, "init", "testapp", "-m", "github.com/test/testapp")
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("juango init failed: %v\n%s", err, output)
	}

	// Read and validate package.json
	packageJSONPath := filepath.Join(tmpDir, "testapp", "frontend", "package.json")
	packageJSONContent, err := os.ReadFile(packageJSONPath)
	if err != nil {
		t.Fatalf("Failed to read package.json: %v", err)
	}

	var packageJSON map[string]interface{}
	if err := json.Unmarshal(packageJSONContent, &packageJSON); err != nil {
		t.Fatalf("Failed to parse package.json: %v", err)
	}

	// Check dependencies
	deps, ok := packageJSON["dependencies"].(map[string]interface{})
	if !ok {
		t.Fatal("Could not find dependencies in package.json")
	}

	// List of dependencies that should NOT exist (unpublished packages)
	forbiddenDeps := []string{
		"@juango/ui",
	}

	for _, forbidden := range forbiddenDeps {
		if _, exists := deps[forbidden]; exists {
			t.Errorf("package.json contains unpublished dependency: %s", forbidden)
		}
	}

	// List of dependencies that SHOULD exist
	requiredDeps := []string{
		"react",
		"react-dom",
		"react-router-dom",
		"@radix-ui/react-slot",
	}

	for _, required := range requiredDeps {
		if _, exists := deps[required]; !exists {
			t.Errorf("package.json missing required dependency: %s", required)
		}
	}
}

// TestScaffoldingFrontendImports tests that frontend files use valid local imports
func TestScaffoldingFrontendImports(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if npm not available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "juango-scaffold-imports-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build and run juango init
	juangoBin := filepath.Join(tmpDir, "juango")
	buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
	buildCmd.Dir = projectRoot()
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build juango: %v\n%s", err, output)
	}

	initCmd := exec.Command(juangoBin, "init", "testapp", "-m", "github.com/test/testapp")
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("juango init failed: %v\n%s", err, output)
	}

	// Files to check for @juango/ui imports
	filesToCheck := []string{
		"frontend/src/App.tsx",
		"frontend/src/pages/Home.tsx",
		"frontend/src/pages/Login.tsx",
		"frontend/src/components/Layout.tsx",
	}

	for _, file := range filesToCheck {
		filePath := filepath.Join(tmpDir, "testapp", file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read %s: %v", file, err)
			continue
		}

		if strings.Contains(string(content), "@juango/ui") {
			t.Errorf("%s contains import from @juango/ui (unpublished package)", file)
		}
	}

	// Verify required files exist
	requiredFiles := []string{
		"frontend/src/contexts/AuthContext.tsx",
		"frontend/src/contexts/AdminContext.tsx",
		"frontend/src/contexts/BreadcrumbContext.tsx",
		"frontend/src/components/ProtectedRoute.tsx",
		"frontend/src/components/ui/button.tsx",
		"frontend/src/components/ui/card.tsx",
		"frontend/src/lib/api.ts",
		"frontend/src/lib/types.ts",
		"frontend/src/lib/utils.ts",
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(tmpDir, "testapp", file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Required file missing: %s", file)
		}
	}
}

// TestScaffoldingProjectStructure validates the overall project structure
func TestScaffoldingProjectStructure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip if npm not available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "juango-scaffold-structure-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Build and run juango init
	juangoBin := filepath.Join(tmpDir, "juango")
	buildCmd := exec.Command("go", "build", "-o", juangoBin, ".")
	buildCmd.Dir = projectRoot()
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build juango: %v\n%s", err, output)
	}

	initCmd := exec.Command(juangoBin, "init", "myproject", "-m", "github.com/test/myproject")
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("juango init failed: %v\n%s", err, output)
	}

	projectDir := filepath.Join(tmpDir, "myproject")

	// Check required directories
	requiredDirs := []string{
		"cmd/myproject",
		"cmd/myproject/cli",
		"internal/api",
		"internal/database",
		"internal/database/sql",
		"internal/types",
		"frontend/src",
		"frontend/src/components",
		"frontend/src/pages",
		"frontend/src/lib",
		"scripts",
	}

	for _, dir := range requiredDirs {
		dirPath := filepath.Join(projectDir, dir)
		if info, err := os.Stat(dirPath); os.IsNotExist(err) || !info.IsDir() {
			t.Errorf("Required directory missing: %s", dir)
		}
	}

	// Check required files
	requiredFiles := []string{
		"go.mod",
		"app.go",
		"config.example.yml",
		".gitignore",
		".goreleaser.yml",
		"cmd/myproject/myproject.go",
		"cmd/myproject/cli/root.go",
		"cmd/myproject/cli/serve.go",
		"internal/api/app.go",
		"internal/database/db.go",
		"internal/database/sql/schema.sql",
		"internal/types/config.go",
		"frontend/package.json",
		"frontend/vite.config.ts",
		"frontend/tsconfig.json",
		"frontend/index.html",
		"frontend/src/App.tsx",
		"frontend/src/main.tsx",
		"scripts/build_frontend.sh",
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(projectDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Required file missing: %s", file)
		}
	}
}
