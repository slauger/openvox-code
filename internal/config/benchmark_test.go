package config

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParseFlat(b *testing.B) {
	data := []byte(`cachedir: /tmp/cache
environmentdir: /tmp/envs
sources:
  - url: https://github.com/example/repo.git
    branchSelector: {}
modulesets:
  base:
    - name: stdlib
      git: https://github.com/puppetlabs/stdlib.git
      ref: v9.0.0
    - name: concat
      git: https://github.com/puppetlabs/concat.git
      ref: v9.0.0
environments:
  production:
    ref: main
    modulesets: [base]
`)
	b.ResetTimer()
	for range b.N {
		_, _ = parse(data)
	}
}

func BenchmarkParseK8sStyle(b *testing.B) {
	data := []byte(`apiVersion: openvox.voxpupuli.org/v1alpha1
kind: CodeConfig
spec:
  cachedir: /tmp/cache
  environmentdir: /tmp/envs
  sources:
    - url: https://github.com/example/repo.git
      branchSelector:
        matchPatterns: ["production", "staging", "feature/*"]
        excludePatterns: ["feature/wip-*"]
`)
	b.ResetTimer()
	for range b.N {
		_, _ = parse(data)
	}
}

func BenchmarkBranchSelectorMatches(b *testing.B) {
	bs := BranchSelector{
		MatchPatterns:   []string{"production", "staging", "feature/*", "release-*"},
		ExcludePatterns: []string{"feature/wip-*", "feature/test-*"},
	}
	branches := []string{"production", "staging", "feature/login", "feature/wip-x", "develop", "release-1.0"}

	b.ResetTimer()
	for range b.N {
		for _, branch := range branches {
			_ = bs.Matches(branch)
		}
	}
}

func BenchmarkValidateLargeConfig(b *testing.B) {
	// Generate config with many modulesets and environments
	var sb strings.Builder
	sb.WriteString("cachedir: /tmp/cache\nenvironmentdir: /tmp/envs\nmodulesets:\n")
	for i := range 10 {
		fmt.Fprintf(&sb, "  set%d:\n", i)
		for j := range 20 {
			fmt.Fprintf(&sb, "    - name: mod%d_%d\n      git: https://example.com/mod%d_%d.git\n      ref: v1.0\n", i, j, i, j)
		}
	}
	sb.WriteString("environments:\n")
	for i := range 50 {
		fmt.Fprintf(&sb, "  env%d:\n    ref: main\n    modulesets: [set0, set1]\n", i)
	}

	data := []byte(sb.String())
	b.ResetTimer()
	for range b.N {
		cfg, _ := parse(data)
		if cfg != nil {
			_ = cfg.Validate()
		}
	}
}
