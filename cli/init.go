package cli

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

//go:embed templates/*
var templates embed.FS

var initFlags struct {
	module      string
	description string
	port        int
}

var initCmd = &cobra.Command{
	Use:   "init <project-name>",
	Short: "Create a new juango project",
	Long: `Creates a new full-stack web application with:
  - Go backend using juango libraries
  - Vite/React frontend
  - SQLite database with WAL mode
  - OIDC authentication ready
  - Admin mode and impersonation support

Example:
  juango init myapp -m github.com/myorg/myapp
  juango init myapp -m gitlab.com/myuser/myapp`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initFlags.module, "module", "m", "", "Go module path (required, e.g. github.com/user/project)")
	initCmd.MarkFlagRequired("module")
	initCmd.Flags().StringVarP(&initFlags.description, "description", "d", "", "Project description")
	initCmd.Flags().IntVarP(&initFlags.port, "port", "p", 8080, "Default port")
}

// TemplateData holds all the data passed to templates
type TemplateData struct {
	ProjectName       string
	ModulePath        string
	Description       string
	Port              int
	ProjectNameTitle  string // Title case version
	ProjectNameLower  string // Lowercase version
	ProjectNameUpper  string // Uppercase version
}

func runInit(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	// Check prerequisites
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found in PATH. Please install Node.js and npm first")
	}
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found in PATH. Please install Go first")
	}

	// Validate project name
	if !isValidProjectName(projectName) {
		return fmt.Errorf("invalid project name: %s (must be lowercase alphanumeric with hyphens)", projectName)
	}

	// Module path is required (enforced by cobra)
	modulePath := initFlags.module

	// Create template data
	data := TemplateData{
		ProjectName:       projectName,
		ModulePath:        modulePath,
		Description:       initFlags.description,
		Port:              initFlags.port,
		ProjectNameTitle:  toTitleCase(projectName),
		ProjectNameLower:  strings.ToLower(projectName),
		ProjectNameUpper:  strings.ToUpper(strings.ReplaceAll(projectName, "-", "_")),
	}

	if data.Description == "" {
		data.Description = fmt.Sprintf("%s - A juango application", data.ProjectNameTitle)
	}

	// Check if directory exists
	if _, err := os.Stat(projectName); !os.IsNotExist(err) {
		return fmt.Errorf("directory %s already exists", projectName)
	}

	fmt.Printf("Creating new juango project: %s\n", projectName)
	fmt.Printf("  Module path: %s\n", modulePath)
	fmt.Printf("  Port: %d\n\n", initFlags.port)

	// Create project directory
	if err := os.MkdirAll(projectName, 0755); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}

	// Generate files from templates
	if err := generateProject(projectName, data); err != nil {
		// Cleanup on error
		os.RemoveAll(projectName)
		return fmt.Errorf("generating project: %w", err)
	}

	// Run npm install
	fmt.Println("Installing frontend dependencies...")
	npmCmd := exec.Command("npm", "install")
	npmCmd.Dir = filepath.Join(projectName, "frontend")
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	if err := npmCmd.Run(); err != nil {
		os.RemoveAll(projectName)
		return fmt.Errorf("npm install failed: %w", err)
	}

	// Run go mod tidy
	fmt.Println("\nTidying Go modules...")
	goCmd := exec.Command("go", "mod", "tidy")
	goCmd.Dir = projectName
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr
	if err := goCmd.Run(); err != nil {
		fmt.Printf("Warning: go mod tidy failed: %v\n", err)
		fmt.Println("You can run 'go mod tidy' manually")
	}

	// Print success message
	fmt.Printf("\n✓ Project created successfully!\n\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  # Configure config.yaml with your OIDC settings\n")
	fmt.Printf("  juango dev\n\n")
	fmt.Printf("Your app will be available at http://localhost:%d\n", initFlags.port)

	return nil
}

func generateProject(projectDir string, data TemplateData) error {
	// Walk through embedded templates
	entries, err := templates.ReadDir("templates")
	if err != nil {
		return err
	}

	return walkTemplates("templates", projectDir, data, entries)
}

func walkTemplates(srcDir, dstDir string, data TemplateData, entries []os.DirEntry) error {
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())

		// Process filename templates
		dstName := processFilename(entry.Name(), data)
		dstPath := filepath.Join(dstDir, dstName)

		if entry.IsDir() {
			// Create directory
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}

			// Recurse into directory
			subEntries, err := templates.ReadDir(srcPath)
			if err != nil {
				return err
			}
			if err := walkTemplates(srcPath, dstPath, data, subEntries); err != nil {
				return err
			}
		} else {
			// Process file
			if err := processTemplate(srcPath, dstPath, data); err != nil {
				return fmt.Errorf("processing %s: %w", srcPath, err)
			}
		}
	}
	return nil
}

func processFilename(name string, data TemplateData) string {
	// Remove .tmpl extension
	name = strings.TrimSuffix(name, ".tmpl")

	// Replace placeholders
	name = strings.ReplaceAll(name, "{{ProjectName}}", data.ProjectName)
	name = strings.ReplaceAll(name, "{{projectname}}", data.ProjectNameLower)

	return name
}

func processTemplate(srcPath, dstPath string, data TemplateData) error {
	content, err := templates.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// If not a .tmpl file, just copy as-is
	if !strings.HasSuffix(srcPath, ".tmpl") {
		return os.WriteFile(dstPath, content, 0644)
	}

	// Parse and execute template
	tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	// Remove .tmpl from destination
	dstPath = strings.TrimSuffix(dstPath, ".tmpl")

	// Create destination file
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func isValidProjectName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return name[0] != '-' && name[len(name)-1] != '-'
}

func toTitleCase(s string) string {
	words := strings.Split(s, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}
	return strings.Join(words, "")
}
