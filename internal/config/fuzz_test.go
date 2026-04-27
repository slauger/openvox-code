package config

import (
	"testing"
)

func FuzzParseConfig(f *testing.F) {
	// Seed corpus with valid configs
	f.Add([]byte(`cachedir: /tmp/cache
environmentdir: /tmp/envs
sources:
  - url: https://github.com/example/repo.git
`))
	f.Add([]byte(`apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /tmp/cache
  environmentdir: /tmp/envs
`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))
	f.Add([]byte(`sources: []`))
	f.Add([]byte(`modulesets:
  base:
    - name: stdlib
      git: https://example.com/stdlib.git
      ref: v1.0.0
`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// parse should never panic, regardless of input
		cfg, err := parse(data)
		if err != nil {
			return
		}
		// If parsing succeeded, Validate should not panic either
		_ = cfg.Validate()
	})
}

func FuzzParseModuleFile(f *testing.F) {
	f.Add([]byte(`modules:
  - name: stdlib
    git: https://example.com/stdlib.git
    ref: v1.0.0
`))
	f.Add([]byte(`apiVersion: openvox.voxpupuli.org/v1alpha1
kind: ModuleFile
spec:
  modules:
    - name: stdlib
      git: https://example.com/stdlib.git
  exclude:
    - deprecated
`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseModuleFile should never panic
		_, _ = ParseModuleFile(data)
	})
}

func FuzzBranchSelectorMatches(f *testing.F) {
	f.Add("main", "main", "")
	f.Add("feature/login", "feature/*", "")
	f.Add("wip-broken", "*", "wip-*")
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, branch, matchPattern, excludePattern string) {
		var matchPatterns, excludePatterns []string
		if matchPattern != "" {
			matchPatterns = []string{matchPattern}
		}
		if excludePattern != "" {
			excludePatterns = []string{excludePattern}
		}
		bs := BranchSelector{
			MatchPatterns:   matchPatterns,
			ExcludePatterns: excludePatterns,
		}
		// Should never panic
		_ = bs.Matches(branch)
		_ = bs.MatchesAll()
	})
}
