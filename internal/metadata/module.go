package metadata

import (
	"strings"
)

// DeriveModuleName splits the last path segment as project name
func DeriveModuleName(moduleURL string) (string, string) {
	// e.g., 'github.com/rrs/shoes' -> name='shoes', package='github.com/rrs/shoes'
	parts := strings.Split(moduleURL, "/")
	name := parts[len(parts)-1]
	return name, moduleURL
}
