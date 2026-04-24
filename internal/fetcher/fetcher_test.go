package fetcher

import (
	"testing"

	"github.com/slauger/openvox-code/internal/resolver"
)

func TestUniqueURLs(t *testing.T) {
	envs := []resolver.ResolvedEnvironment{
		{
			Name:           "production",
			ControlRepoURL: "https://github.com/example/control.git",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://github.com/puppetlabs/stdlib.git"},
				{Name: "concat", GitURL: "https://github.com/puppetlabs/concat.git"},
			},
		},
		{
			Name:           "staging",
			ControlRepoURL: "https://github.com/example/control.git",
			Modules: []resolver.ResolvedModule{
				{Name: "stdlib", GitURL: "https://github.com/puppetlabs/stdlib.git"},
				{Name: "apache", GitURL: "https://github.com/puppetlabs/apache.git"},
			},
		},
	}

	urls := uniqueURLs(envs)

	// Should have 4 unique URLs: control, stdlib, concat, apache
	if len(urls) != 4 {
		t.Errorf("uniqueURLs() returned %d URLs, want 4", len(urls))
	}

	seen := make(map[string]bool)
	for _, u := range urls {
		if seen[u] {
			t.Errorf("duplicate URL in result: %s", u)
		}
		seen[u] = true
	}
}

func TestUniqueURLsEmpty(t *testing.T) {
	urls := uniqueURLs(nil)
	if len(urls) != 0 {
		t.Errorf("uniqueURLs(nil) returned %d URLs, want 0", len(urls))
	}
}
