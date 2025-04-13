package templates

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// TemplateData only contains the project configuration fields
// that should be replaced during template generation
type TemplateData struct {
	ProjectName string
	ModuleURL   string
	HomeDir     string
	MainPath    string
}

// taskVarMap is a map of task variables that should be preserved in the output
type taskVarMap map[string]string

// Get implements a custom getter for task variables that returns the variable reference
func (t taskVarMap) Get(name string) string {
	return fmt.Sprintf("{{.%s}}", name)
}

func TestTemplates(t *testing.T) {
	// Create a temporary output directory
	outputDir := "../testout"
	if err := os.RemoveAll(outputDir); err != nil {
		t.Fatalf("failed to clean output dir: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	// Create package directories
	for _, dir := range []string{"cmd/testapp", "logs", "config"} {
		if err := os.MkdirAll(filepath.Join(outputDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Initialize a new module
	if err := os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(`
module github.com/example/testapp

go 1.21

require (
	github.com/jessevdk/go-flags v1.5.0
	go.uber.org/zap v1.26.0
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/example/testapp/config => ./config
	github.com/example/testapp/logs => ./logs
)
`), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	testCases := []struct {
		name       string
		tmplFile   string
		outputFile string
		data       struct {
			TemplateData
			Task taskVarMap
		}
	}{
		{
			name:       "taskfile template",
			tmplFile:   "taskfile.tmpl",
			outputFile: "Taskfile.yaml",
			data: struct {
				TemplateData
				Task taskVarMap
			}{
				TemplateData: TemplateData{
					ProjectName: "testapp",
					MainPath:    "./cmd/testapp",
				},
				Task: taskVarMap{
					"VERSION":   "",
					"COMMIT":    "",
					"BUILDTIME": "",
					"LDFLAGS":   "",
					"OUT":       "",
					"MAIN":      "",
					"CLI_ARGS":  "",
				},
			},
		},
		{
			name:       "main template",
			tmplFile:   "main.tmpl",
			outputFile: "cmd/testapp/main.go",
			data: struct {
				TemplateData
				Task taskVarMap
			}{
				TemplateData: TemplateData{
					ProjectName: "testapp",
				},
			},
		},
		{
			name:       "config template",
			tmplFile:   "config.tmpl",
			outputFile: "config/config.go",
			data: struct {
				TemplateData
				Task taskVarMap
			}{
				TemplateData: TemplateData{
					ProjectName: "testapp",
					HomeDir:     "~/testapp",
				},
			},
		},
		{
			name:       "logs template",
			tmplFile:   "logs.tmpl",
			outputFile: "logs/logs.go",
			data: struct {
				TemplateData
				Task taskVarMap
			}{
				TemplateData: TemplateData{
					ProjectName: "testapp",
				},
			},
		},
	}

	// Process each template
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read template
			tmplContent, err := os.ReadFile(tc.tmplFile)
			if err != nil {
				t.Fatalf("failed to read template: %v", err)
			}

			// For taskfile.tmpl, replace Task variables with our taskVar function
			content := string(tmplContent)
			if tc.tmplFile == "taskfile.tmpl" {
				replacements := []struct{ old, new string }{
					{`{{ "{{.VERSION}}" }}`, `{{.Task.Get "VERSION"}}`},
					{`{{ "{{.COMMIT}}" }}`, `{{.Task.Get "COMMIT"}}`},
					{`{{ "{{.BUILDTIME}}" }}`, `{{.Task.Get "BUILDTIME"}}`},
					{`{{ "{{.LDFLAGS}}" }}`, `{{.Task.Get "LDFLAGS"}}`},
					{`{{ "{{.OUT}}" }}`, `{{.Task.Get "OUT"}}`},
					{`{{ "{{.MAIN}}" }}`, `{{.Task.Get "MAIN"}}`},
					{`{{ "{{.CLI_ARGS}}" }}`, `{{.Task.Get "CLI_ARGS"}}`},
					// Also replace direct Task variables in the vars section
					{`.VERSION`, `.Task.Get "VERSION"`},
					{`.COMMIT`, `.Task.Get "COMMIT"`},
					{`.BUILDTIME`, `.Task.Get "BUILDTIME"`},
					{`.LDFLAGS`, `.Task.Get "LDFLAGS"`},
					{`.OUT`, `.Task.Get "OUT"`},
					{`.MAIN`, `.Task.Get "MAIN"`},
					{`.CLI_ARGS`, `.Task.Get "CLI_ARGS"`},
				}
				for _, r := range replacements {
					content = strings.ReplaceAll(content, r.old, r.new)
				}
			}

			// Parse template
			tmpl, err := template.New(tc.tmplFile).Parse(content)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			// Execute template
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tc.data); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}
			output := buf.String()

			// Write the generated file
			if err := os.WriteFile(filepath.Join(outputDir, tc.outputFile), []byte(output), 0644); err != nil {
				t.Fatalf("failed to write output file: %v", err)
			}

			// Verify output
			switch tc.tmplFile {
			case "taskfile.tmpl":
				if !strings.Contains(output, tc.data.ProjectName) {
					t.Errorf("taskfile output missing project name")
				}
			case "main.tmpl":
				if !strings.Contains(output, tc.data.ProjectName) {
					t.Errorf("main output missing project name")
				}
			case "config.tmpl":
				if !strings.Contains(output, tc.data.ProjectName) {
					t.Errorf("config output missing project name")
				}
			case "logs.tmpl":
				if !strings.Contains(output, tc.data.ProjectName) {
					t.Errorf("logs output missing project name")
				}
			}
		})
	}

	// After all files are written, check if .gitignore exists and append bin/* if needed
	gitignorePath := filepath.Join(outputDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		// .gitignore exists, check if bin/* is already in it
		content, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		if !strings.Contains(string(content), "bin/*") {
			// Append bin/* to .gitignore
			f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				t.Fatalf("failed to open .gitignore for append: %v", err)
			}
			if _, err := f.WriteString("\n# Binary output directory\nbin/*\n"); err != nil {
				f.Close()
				t.Fatalf("failed to append to .gitignore: %v", err)
			}
			f.Close()
		}
	}

	// After all files are written, initialize the module
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = outputDir
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to initialize module: %v\n%s", err, output)
	}

	// Try to compile all packages
	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = outputDir
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code failed to compile: %v\n%s", err, output)
	}
}
