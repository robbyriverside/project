package project

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type GenConfig struct {
	ModuleURL   string
	ProjectName string
	OutputDir   string

	// HomeDir is a default value referencing the project name,
	// e.g. "~/shoes" if ProjectName="shoes"
	HomeDir string
}

// NewGenConfig derives ProjectName from the module URL, sets outDir to "." if empty,
// and defaults HomeDir to "~/{ProjectName}"
func NewGenConfig(moduleURL, outDir string) *GenConfig {
	if outDir == "" {
		outDir = "."
	}
	parts := strings.Split(strings.TrimSpace(moduleURL), "/")
	name := parts[len(parts)-1]

	return &GenConfig{
		ModuleURL:   moduleURL,
		ProjectName: name,
		OutputDir:   outDir,
		HomeDir:     fmt.Sprintf("~/%s", name),
	}
}

// ProjectPath returns the absolute path where the new project folder goes.
func (gc *GenConfig) ProjectPath() string {
	abs, err := filepath.Abs(gc.OutputDir)
	if err != nil {
		abs = gc.OutputDir // fallback
	}
	return abs
}

// Generator coordinates the template lookups and file generation.
type Generator struct {
	Config *GenConfig
}

func (g *Generator) readTemplate(name string) (*template.Template, error) {
	fsys, err := fs.Sub(templateFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", name, err)
	}
	content, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	return tmpl, nil
}

// GenerateAll creates the config and runs each file generation plus go mod steps.
func (g *Generator) GenerateAll(moduleURL, outDir string) error {
	// Build or update the config
	g.Config = NewGenConfig(moduleURL, outDir)

	// Add any file types you want to generate:
	fileTypes := []string{"main", "config", "logs", "project", "taskfile"}

	for _, ft := range fileTypes {
		if err := g.GenerateFile(ft); err != nil {
			return fmt.Errorf("failed to generate %s: %w", ft, err)
		}
	}

	// Post-process Taskfile.yaml
	if err := g.postProcessTaskfile(); err != nil {
		return fmt.Errorf("failed to post-process Taskfile.yaml: %w", err)
	}

	// Finally do go mod init + tidy
	if err := g.InitMod(); err != nil {
		return fmt.Errorf("go mod init failed: %w", err)
	}
	if err := g.addReplaceDirectives(); err != nil {
		return fmt.Errorf("failed to add replace directives: %w", err)
	}
	if err := g.ModTidy(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}

// GenerateFile reads <fileType>.tmpl, executes it with g.Config, writes the result.
func (g *Generator) GenerateFile(fileType string) error {
	tplName := fileType + ".tmpl"
	tpl, err := g.readTemplate(tplName)
	if err != nil {
		return err
	}

	// Execute
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, g.Config); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", fileType, err)
	}

	// Determine final output path
	destPath := g.filePath(fileType)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to mkdir for %s: %w", destPath, err)
	}

	// Write result
	if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}

	return nil
}

// filePath chooses the output location for each type of file.
func (g *Generator) filePath(fileType string) string {
	projPath := g.Config.ProjectPath()

	switch fileType {
	case "main":
		return filepath.Join(projPath, "cmd", g.Config.ProjectName, "main.go")
	case "config":
		return filepath.Join(projPath, "config", "config.go")
	case "logs":
		return filepath.Join(projPath, "logs", "logs.go")
	case "taskfile":
		return filepath.Join(projPath, "Taskfile.yaml")
	case "project":
		return filepath.Join(projPath, g.Config.ProjectName+".go")
	default:
		return filepath.Join(projPath, fileType+".go")
	}
}

// postProcessTaskfile replaces .Task.Get calls with simpler variables in the generated Taskfile.yaml
func (g *Generator) postProcessTaskfile() error {
	taskfilePath := filepath.Join(g.Config.ProjectPath(), "Taskfile.yaml")
	content, err := os.ReadFile(taskfilePath)
	if err != nil {
		return fmt.Errorf("failed to read Taskfile: %w", err)
	}

	// Replace all .Task.Get calls with simple variables
	replacements := []struct{ old, new string }{
		{`{{.Task.Get "VERSION"}}`, "{{.VERSION}}"},
		{`{{.Task.Get "COMMIT"}}`, "{{.COMMIT}}"},
		{`{{.Task.Get "BUILDTIME"}}`, "{{.BUILDTIME}}"},
		{`{{.Task.Get "MAIN"}}`, "{{.MAIN}}"},
		{`{{.Task.Get "CLI_ARGS"}}`, "{{.CLI_ARGS}}"},
		{`{{.Task.Get "OUT"}}`, "{{.OUT}}"},
		{`{{.Task.Get "LDFLAGS"}}`, "{{.LDFLAGS}}"},
		{`{{.Task.Get "APP"}}`, "{{.APP}}"},
	}

	newContent := string(content)
	for _, r := range replacements {
		newContent = strings.ReplaceAll(newContent, r.old, r.new)
	}

	if err := os.WriteFile(taskfilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write Taskfile: %w", err)
	}

	return nil
}

// InitMod runs `go mod init <moduleURL>` in the project folder
func (g *Generator) InitMod() error {
	pp := g.Config.ProjectPath()
	modPath := filepath.Join(pp, "go.mod")
	
	// Check if go.mod already exists
	if _, err := os.Stat(modPath); err == nil {
		// go.mod exists, skip initialization
		return nil
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to check for go.mod: %w", err)
	}
	
	cmd := exec.Command("go", "mod", "init", g.Config.ModuleURL)
	cmd.Dir = pp
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go mod init: %w", err)
	}
	return nil
}

// addReplaceDirectives adds replace directives to go.mod for local packages
func (g *Generator) addReplaceDirectives() error {
	pp := g.Config.ProjectPath()
	modPath := filepath.Join(pp, "go.mod")

	content, err := os.ReadFile(modPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	// Add replace directives if they don't exist
	replaces := fmt.Sprintf(`

replace (
	%[1]s => .
	%[1]s/config => ./config
	%[1]s/logs => ./logs
)
`, g.Config.ModuleURL)

	if !strings.Contains(string(content), "replace (") {
		newContent := string(content) + replaces
		if err := os.WriteFile(modPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write go.mod: %w", err)
		}
	}

	return nil
}

// ModTidy runs `go mod tidy` in the project folder
func (g *Generator) ModTidy() error {
	pp := g.Config.ProjectPath()
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = pp
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
