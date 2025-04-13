// package config provides a reflective configuration system
// for the generated CLI.
//
// Usage:
//
//	config.Load()    // reads ~/.myapp/config.yaml
//	config.Save(cfg) // writes it back
//	config.Set("home", "/path/to")  // modifies a key
//	config.Get("home")              // retrieves a key
//	lines, _ := config.Describe()   // shows current fields + descriptions
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all user-facing config fields.
// Each field is annotated with YAML plus a custom 'config' tag
// that includes desc= and default= pairs for reflection in Describe().
type Config struct {
	HomeDir string `yaml:"home" config:"desc=Base directory for storing data,default=~/myapp"`
	Author  string `yaml:"author" config:"desc=Default author name for new items"`
	LogFmt  string `yaml:"log_fmt" config:"desc=Log output format (json, formatted, text),default=json"`
}

// defaultConfig includes built-in fallback fields (like user name).
var defaultConfig = Config{
	HomeDir: "~/myapp",
	LogFmt:  "json",
	Author:  fallbackAuthor(),
}

// fallbackAuthor tries to glean a user name from the environment or OS user.
func fallbackAuthor() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	// fallback to directory name of $HOME
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Base(home)
	}
	return "unknown"
}

// Path returns the location of the config file. Edit for your app name.
func Path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".myapp/config.yaml"
	}
	return filepath.Join(home, ".myapp", "config.yaml")
}

// Load loads the config file or returns defaults if it's missing.
func Load() (*Config, error) {
	path := Path()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if none on disk
			cfg := defaultConfig
			return &cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := defaultConfig // allow defaults to fill in missing fields
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config back to disk.
func Save(cfg *Config) error {
	path := Path()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to make config dir: %w", err)
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// Set modifies one field in the config, saving immediately.
func Set(key, value string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(cfg).Elem()
	rt := rv.Type()

	found := false
	for i := 0; i < rv.NumField(); i++ {
		yamlTag := rt.Field(i).Tag.Get("yaml")
		if yamlTag == key {
			// If setting 'home', convert to absolute path
			if key == "home" {
				absPath, err := filepath.Abs(value)
				if err == nil {
					value = absPath
				}
			}
			rv.Field(i).SetString(value)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown config key: %s", key)
	}

	return Save(cfg)
}

// Get retrieves one field's value from the config.
func Get(key string) (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	rv := reflect.ValueOf(cfg).Elem()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		yamlTag := rt.Field(i).Tag.Get("yaml")
		if yamlTag == key {
			return rv.Field(i).String(), nil
		}
	}
	return "", fmt.Errorf("unknown config key: %s", key)
}

// Describe returns a human-readable listing of config fields,
// showing the current value, default, and a short description.
func Describe() ([]string, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	rv := reflect.ValueOf(cfg).Elem()
	rt := rv.Type()

	var out []string
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		yamlTag := field.Tag.Get("yaml")
		descTag := field.Tag.Get("config")

		parts := parseTag(descTag)
		value := rv.Field(i).String()

		// If empty, use default from config tag
		if strings.TrimSpace(value) == "" && parts["default"] != "" {
			value = parts["default"]
		}
		desc := parts["desc"]

		out = append(out, fmt.Sprintf("  %s = %s\n    → %s", yamlTag, value, desc))
	}
	return out, nil
}

// DescribeJSON returns a JSON representation of each config field
// with { fieldKey: {value, desc, default} }
func DescribeJSON() ([]byte, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	rv := reflect.ValueOf(cfg).Elem()
	rt := rv.Type()

	type fieldMeta struct {
		Value   string `json:"value"`
		Desc    string `json:"desc"`
		Default string `json:"default"`
	}

	results := make(map[string]fieldMeta)
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		yamlKey := field.Tag.Get("yaml")
		tag := field.Tag.Get("config")
		parts := parseTag(tag)

		val := rv.Field(i).String()
		if val == "" {
			val = parts["default"]
		}
		results[yamlKey] = fieldMeta{
			Value:   val,
			Desc:    parts["desc"],
			Default: parts["default"],
		}
	}

	return json.MarshalIndent(results, "", "  ")
}

// parseTag splits the config tag e.g. 'desc=...,default=...' into a map.
func parseTag(tag string) map[string]string {
	out := make(map[string]string)
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}
