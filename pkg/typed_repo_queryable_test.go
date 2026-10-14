package nitro

import (
	"testing"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestAdaptRepo_MemoryNotQueryableLister(t *testing.T) {
	repo := AdaptRepo(NewMemoryRepo[struct {
		ID string `json:"id"`
	}]())
	if _, ok := repo.(domain.QueryableLister); ok {
		t.Fatal("MemoryRepo AdaptRepo must not implement QueryableLister (tenant/FilterRules post-filter)")
	}
}

func TestCacheRepo_MemoryNotQueryableLister(t *testing.T) {
	repo := CacheRepo(AdaptRepo(NewMemoryRepo[struct {
		ID string `json:"id"`
	}]()), DefaultCacheTTL)
	if _, ok := repo.(domain.QueryableLister); ok {
		t.Fatal("CacheRepo over MemoryRepo must not implement QueryableLister")
	}
	if !IsCacheRepo(repo) {
		t.Fatal("IsCacheRepo")
	}
}
