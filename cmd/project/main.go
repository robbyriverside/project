package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"

	// Hypothetical references to your config, logs packages, etc.
	"github.com/robbyriverside/project"
	config "github.com/robbyriverside/project/config"
	logs "github.com/robbyriverside/project/logs"
)

// Top-level CLI options
type Options struct {
	Verbose bool `short:"v" long:"verbose" description:"Enable verbose logging"`
}

func main() {
	var opts Options
	parser := flags.NewParser(&opts, flags.Default)

	parser.AddCommand("gen",
		"Generate a new Go CLI project",
		"Scaffolds a baseline CLI with config, logs, etc.",
		&GenCommand{},
	)

	// 1) Register the parent 'config' command
	cfgParser, _ := parser.AddCommand(
		"config",
		"Manage configuration",
		"View or modify your config settings",
		&ConfigCommand{},
	)

	// 2) Add subcommands on the 'cfgCmd' subcommand parser
	cfgParser.AddCommand("describe", "Show config file location and values", "",
		&ConfigDescribeCommand{})
	cfgParser.AddCommand("set", "Set a config key", "",
		&ConfigSetCommand{})
	cfgParser.AddCommand("get", "Get a config key", "",
		&ConfigGetCommand{})

	// Example: version command
	parser.AddCommand("version", "Show version info", "",
		&VersionCommand{})

	// Parse
	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	// After parse, set up logging
	logs.Options.Verbose = opts.Verbose
	logs.InitLogger(os.Getenv("ENV"))

	logs.Logger().Info("CLI started. All set.")
}

// ---------------------------------------------------------------------
// config parent

type ConfigCommand struct{}

func (cmd *ConfigCommand) Execute(args []string) error {
	if len(args) == 0 {
		// If user just runs 'fibber config' with no subcommand
		return fmt.Errorf("please specify a subcommand: describe, set, or get")
	}
	return nil // let subcommand logic run
}

// ---------------------------------------------------------------------
// config describe

type ConfigDescribeCommand struct{}

func (cmd *ConfigDescribeCommand) Execute(args []string) error {
	fmt.Println("Fibber config file:", config.Path())

	lines, err := config.Describe()
	if err != nil {
		return err
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

// gen command
type GenCommand struct {
	// A required positional argument for the GitHub URL, e.g. "https://github.com/rrs/shoes"
	Args struct {
		GitURL string `positional-arg-name:"gitURL" required:"true" description:"GitHub URL (same as git clone URL)"`
	} `positional-args:"yes"`

	// An optional flag to override the output directory, defaults to repo name
	Dir string `short:"d" long:"dir" description:"Output directory (defaults to repository name)"`
}

func (cmd *GenCommand) Execute(args []string) error {
	// Convert GitHub URL to module URL and get repo name
	// Examples:
	// https://github.com/user/repo.git -> github.com/user/repo
	// git@github.com:user/repo.git -> github.com/user/repo
	gitURL := cmd.Args.GitURL
	moduleURL := gitURL
	var repoName string

	// Handle https:// URLs
	if strings.HasPrefix(gitURL, "https://github.com/") {
		moduleURL = strings.TrimPrefix(gitURL, "https://")
	}

	// Handle git@ URLs
	if strings.HasPrefix(gitURL, "git@github.com:") {
		moduleURL = strings.Replace(strings.TrimPrefix(gitURL, "git@"), ":", "/", 1)
	}

	// Remove .git suffix if present
	moduleURL = strings.TrimSuffix(moduleURL, ".git")

	// Extract repository name from moduleURL
	parts := strings.Split(moduleURL, "/")
	if len(parts) >= 3 {
		repoName = parts[len(parts)-1]
	} else {
		return fmt.Errorf("invalid GitHub URL format")
	}

	// Use repository name as output directory if --dir not specified
	outputDir := cmd.Dir
	if outputDir == "" {
		outputDir = repoName
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create your Generator with a TmplDir pointing to where your .tmpl files live
	gen := &project.Generator{
		Config: project.NewGenConfig(moduleURL, outputDir),
	}

	// Call GenerateAll with the processed moduleURL & dir
	if err := gen.GenerateAll(moduleURL, outputDir); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
	}

	fmt.Printf("Project generated in ./%s\nModule URL: %s\n", outputDir, moduleURL)
	return nil
}

// ---------------------------------------------------------------------
// config set

type ConfigSetCommand struct {
	Args struct {
		Key   string `positional-arg-name:"key" required:"true"`
		Value string `positional-arg-name:"value" required:"true"`
	} `positional-args:"yes"`
}

func (cmd *ConfigSetCommand) Execute(args []string) error {
	if err := config.Set(cmd.Args.Key, cmd.Args.Value); err != nil {
		return fmt.Errorf("error setting config key '%s': %w", cmd.Args.Key, err)
	}

	val, _ := config.Get(cmd.Args.Key)
	fmt.Printf("%s = %s\n", cmd.Args.Key, val)
	return nil
}

// ---------------------------------------------------------------------
// config get

type ConfigGetCommand struct {
	Args struct {
		Key string `positional-arg-name:"key" required:"true"`
	} `positional-args:"yes"`
}

func (cmd *ConfigGetCommand) Execute(args []string) error {
	val, err := config.Get(cmd.Args.Key)
	if err != nil {
		return err
	}
	fmt.Println(val)
	return nil
}

// ---------------------------------------------------------------------
// version

type VersionCommand struct{}

func (cmd *VersionCommand) Execute(args []string) error {
	fmt.Println("Project CLI - version 0.0.1 (dev)")
	return nil
}
