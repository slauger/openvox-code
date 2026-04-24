// Package fetcher handles parallel Git fetch operations for repository caching.
package fetcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/slauger/openvox-code/internal/cache"
	"github.com/slauger/openvox-code/internal/resolver"
)

// Fetcher handles parallel Git fetch operations.
type Fetcher struct {
	cache    *cache.Manager
	parallel int
	log      *slog.Logger
}

// New creates a new Fetcher.
func New(cm *cache.Manager, parallel int, log *slog.Logger) *Fetcher {
	if parallel < 1 {
		parallel = 1
	}
	return &Fetcher{
		cache:    cm,
		parallel: parallel,
		log:      log,
	}
}

// FetchAll fetches all unique Git repositories referenced in the resolved environments.
func (f *Fetcher) FetchAll(ctx context.Context, envs []resolver.ResolvedEnvironment) error {
	urls := uniqueURLs(envs)
	if len(urls) == 0 {
		f.log.Info("no repositories to fetch")
		return nil
	}

	f.log.Info("fetching repositories", "count", len(urls), "parallel", f.parallel)

	var (
		wg      sync.WaitGroup
		errChan = make(chan error, len(urls))
		sem     = make(chan struct{}, f.parallel)
	)

	for _, u := range urls {
		wg.Add(1)
		go func(gitURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			f.log.Debug("fetching", "url", gitURL)
			if err := f.cache.EnsureClone(ctx, gitURL); err != nil {
				errChan <- fmt.Errorf("fetching %s: %w", gitURL, err)
			}
		}(u)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("fetch errors (%d): %v", len(errs), errs[0])
	}
	return nil
}

func uniqueURLs(envs []resolver.ResolvedEnvironment) []string {
	seen := make(map[string]bool)
	var urls []string

	for _, env := range envs {
		for _, mod := range env.Modules {
			if !seen[mod.GitURL] {
				seen[mod.GitURL] = true
				urls = append(urls, mod.GitURL)
			}
		}
		if env.ControlRepoURL != "" && !seen[env.ControlRepoURL] {
			seen[env.ControlRepoURL] = true
			urls = append(urls, env.ControlRepoURL)
		}
	}
	return urls
}
