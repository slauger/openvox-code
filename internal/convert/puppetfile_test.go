package convert

import (
	"strings"
	"testing"
)

func TestParsePuppetfile(t *testing.T) {
	input := `# Managed by r10k
mod 'puppetlabs-stdlib', '9.7.0'

mod 'apache',
  :git => 'https://github.com/puppetlabs/puppetlabs-apache.git',
  :tag => 'v12.2.0'

mod 'profiles',
  :git            => 'https://github.com/example/puppet-profiles.git',
  :default_branch => 'main'

mod 'roles',
  :git    => 'https://github.com/example/puppet-roles.git',
  :branch => 'production'

mod 'site_utils',
  :git          => 'https://github.com/example/site-utils.git',
  :commit       => 'abc123def456',
  :install_path => 'site-modules'
`

	modules, err := ParsePuppetfile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePuppetfile error = %v", err)
	}

	if len(modules) != 5 {
		t.Fatalf("got %d modules, want 5", len(modules))
	}

	// Forge module
	if modules[0].Name != "stdlib" {
		t.Errorf("modules[0].Name = %q, want stdlib", modules[0].Name)
	}
	if modules[0].ForgeVersion != "9.7.0" {
		t.Errorf("modules[0].ForgeVersion = %q, want 9.7.0", modules[0].ForgeVersion)
	}

	// Git module with tag
	if modules[1].GitURL != "https://github.com/puppetlabs/puppetlabs-apache.git" {
		t.Errorf("modules[1].GitURL = %q", modules[1].GitURL)
	}
	if modules[1].Ref != "v12.2.0" {
		t.Errorf("modules[1].Ref = %q, want v12.2.0", modules[1].Ref)
	}

	// default_branch
	if modules[2].DefaultBranch != "main" {
		t.Errorf("modules[2].DefaultBranch = %q, want main", modules[2].DefaultBranch)
	}

	// branch
	if modules[3].Ref != "production" {
		t.Errorf("modules[3].Ref = %q, want production", modules[3].Ref)
	}

	// install_path + commit
	if modules[4].Ref != "abc123def456" {
		t.Errorf("modules[4].Ref = %q, want abc123def456", modules[4].Ref)
	}
	if modules[4].InstallPath != "site-modules" {
		t.Errorf("modules[4].InstallPath = %q, want site-modules", modules[4].InstallPath)
	}
}

func TestParsePuppetfileEmpty(t *testing.T) {
	modules, err := ParsePuppetfile(strings.NewReader(""))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("got %d modules, want 0", len(modules))
	}
}

func TestParsePuppetfileCommentsOnly(t *testing.T) {
	input := "# just comments\n# nothing here\n"
	modules, err := ParsePuppetfile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(modules) != 0 {
		t.Errorf("got %d modules, want 0", len(modules))
	}
}

func TestToModuleFileYAML(t *testing.T) {
	modules := []PuppetfileModule{
		{Name: "stdlib", ForgeVersion: "9.7.0"},
		{Name: "apache", GitURL: "https://github.com/puppetlabs/apache.git", Ref: "v12.0"},
		{Name: "profiles", GitURL: "https://example.com/profiles.git", DefaultBranch: "main"},
		{Name: "utils", GitURL: "https://example.com/utils.git", Ref: "v1", InstallPath: "site"},
	}

	data, err := ToModuleFileYAML(modules)
	if err != nil {
		t.Fatalf("ToModuleFileYAML error = %v", err)
	}

	yaml := string(data)

	// Should have K8s-style envelope
	if !strings.Contains(yaml, "apiVersion: openvox.voxpupuli.org/v1alpha1") {
		t.Error("missing apiVersion")
	}
	if !strings.Contains(yaml, "kind: ModuleFile") {
		t.Error("missing kind")
	}

	// Should have warning for Forge module
	if !strings.Contains(yaml, "WARNING: Forge module") {
		t.Error("missing Forge module warning")
	}

	// follow_branch for default_branch
	if !strings.Contains(yaml, "follow_branch: true") {
		t.Error("missing follow_branch for default_branch module")
	}

	// target_dir for install_path
	if !strings.Contains(yaml, "target_dir: site") {
		t.Error("missing target_dir for install_path module")
	}
}

func TestCleanModName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"puppetlabs-stdlib", "stdlib"},
		{"puppetlabs/stdlib", "stdlib"},
		{"stdlib", "stdlib"},
		{"example-my_module", "my_module"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanModName(tt.input)
			if got != tt.want {
				t.Errorf("cleanModName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractQuoted(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{`mod 'stdlib', '9.7.0'`, 2},
		{`mod "stdlib"`, 1},
		{`mod 'name',`, 1},
		{`:git => 'https://example.com/repo.git'`, 1},
		{`no quotes here`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractQuoted(tt.input)
			if len(got) != tt.want {
				t.Errorf("extractQuoted(%q) returned %d items, want %d: %v", tt.input, len(got), tt.want, got)
			}
		})
	}
}
