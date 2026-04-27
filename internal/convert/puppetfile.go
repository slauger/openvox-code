// Package convert handles conversion from other configuration formats to openvox-code YAML.
package convert

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/slauger/openvox-code/internal/config"
	"gopkg.in/yaml.v3"
)

// PuppetfileModule represents a parsed module from a Puppetfile.
type PuppetfileModule struct {
	Name          string
	GitURL        string
	Ref           string
	DefaultBranch string
	InstallPath   string
	ForgeVersion  string // non-empty if this is a Forge module (not Git)
}

// ParsePuppetfile parses a Ruby-style Puppetfile into a list of modules.
// It handles the common patterns but not arbitrary Ruby code.
func ParsePuppetfile(r io.Reader) ([]PuppetfileModule, error) {
	var modules []PuppetfileModule
	scanner := bufio.NewScanner(r)

	var current *PuppetfileModule
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// New mod declaration
		if strings.HasPrefix(line, "mod ") {
			if current != nil {
				modules = append(modules, *current)
			}
			current = parseModLine(line)
			continue
		}

		// Continuation of a mod with :key => 'value'
		if current != nil && strings.Contains(line, "=>") {
			parseModOption(current, line)
		}
	}

	if current != nil {
		modules = append(modules, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading Puppetfile: %w", err)
	}

	return modules, nil
}

// ToModuleFileYAML converts parsed Puppetfile modules to openvox-code ModuleFile YAML.
func ToModuleFileYAML(modules []PuppetfileModule) ([]byte, error) {
	type output struct {
		APIVersion string                `yaml:"apiVersion"`
		Kind       string                `yaml:"kind"`
		Spec       config.ModuleFileSpec `yaml:"spec"`
	}

	var cfgModules []config.Module
	var warnings []string

	for _, m := range modules {
		if m.ForgeVersion != "" {
			warnings = append(warnings, fmt.Sprintf("# WARNING: Forge module %q (%s) — replace with Git URL", m.Name, m.ForgeVersion))
			cfgModules = append(cfgModules, config.Module{
				Name: m.Name,
				Git:  fmt.Sprintf("https://github.com/puppetlabs/puppetlabs-%s.git", m.Name),
				Ref:  "v" + m.ForgeVersion,
			})
			continue
		}

		mod := config.Module{
			Name: m.Name,
			Git:  m.GitURL,
			Ref:  m.Ref,
		}

		if m.DefaultBranch != "" {
			mod.FollowBranch = true
			if mod.Ref == "" {
				mod.Ref = m.DefaultBranch
			}
		}

		if m.InstallPath != "" {
			mod.TargetDir = m.InstallPath
		}

		cfgModules = append(cfgModules, mod)
	}

	doc := output{
		APIVersion: config.APIVersion,
		Kind:       config.KindModuleFile,
		Spec: config.ModuleFileSpec{
			Modules: cfgModules,
		},
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshaling YAML: %w", err)
	}

	// Prepend warnings as comments
	if len(warnings) > 0 {
		header := strings.Join(warnings, "\n") + "\n"
		data = append([]byte(header), data...)
	}

	return data, nil
}

// parseModLine parses "mod 'name', 'version'" or "mod 'name',"
func parseModLine(line string) *PuppetfileModule {
	// Remove trailing comma
	line = strings.TrimSuffix(strings.TrimSpace(line), ",")

	// Extract quoted strings
	parts := extractQuoted(line)
	if len(parts) == 0 {
		return &PuppetfileModule{}
	}

	mod := &PuppetfileModule{Name: cleanModName(parts[0])}

	// If second arg is a version string (not a symbol), it's a Forge module
	if len(parts) >= 2 && !strings.HasPrefix(parts[1], ":") {
		mod.ForgeVersion = parts[1]
	}

	return mod
}

// parseModOption parses ":key => 'value'" lines
func parseModOption(mod *PuppetfileModule, line string) {
	line = strings.TrimSuffix(strings.TrimSpace(line), ",")
	parts := strings.SplitN(line, "=>", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")

	switch key {
	case ":git":
		mod.GitURL = value
	case ":tag", ":ref", ":commit":
		mod.Ref = value
	case ":branch":
		mod.Ref = value
	case ":default_branch":
		mod.DefaultBranch = value
	case ":install_path":
		mod.InstallPath = value
	}
}

// extractQuoted extracts single or double quoted strings from a line.
func extractQuoted(line string) []string {
	var results []string
	inQuote := false
	quoteChar := byte(0)
	var current strings.Builder

	for i := range len(line) {
		c := line[i]
		switch {
		case !inQuote && (c == '\'' || c == '"'):
			inQuote = true
			quoteChar = c
			current.Reset()
		case inQuote && c == quoteChar:
			inQuote = false
			results = append(results, current.String())
		case inQuote:
			current.WriteByte(c)
		}
	}
	return results
}

// cleanModName removes author prefix from "author-name" or "author/name" format.
func cleanModName(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	if i := strings.LastIndex(name, "-"); i >= 0 {
		return name[i+1:]
	}
	return name
}
