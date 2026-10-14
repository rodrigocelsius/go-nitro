package nitro

import (
	"context"
	"sync"
	"testing"
	"time"
)

type raceTodo struct {
	ID        string
	Title     string
	CreatedAt time.Time
	DeletedAt *time.Time
}

// TestMemoryRepository_ConcurrentAccess is a regression test: soft delete and
// update used to mutate the stored entity in place, racing with readers.
func TestMemoryRepository_ConcurrentAccess(t *testing.T) {
	repo := NewMemoryRepo[raceTodo]()
	ctx := context.Background()

	for i := 0; i < 8; i++ {
		if err := repo.Create(ctx, &raceTodo{ID: string(rune('a' + i)), Title: "t"}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				id := string(rune('a' + (i+w)%8))
				switch w % 4 {
				case 0:
					e, err := repo.FindByID(ctx, id)
					if err == nil {
						_ = e.Title // read while others write must not race
					}
				case 1:
					_ = repo.Update(ctx, &raceTodo{ID: id, Title: "u"})
				case 2:
					list, err := repo.List(ctx, 1, 100)
					if err == nil && len(list.Items) > 0 {
						_ = list.Items[0]
					}
				case 3:
					_ = repo.Delete(ctx, id) // soft delete path
					_ = repo.Create(ctx, &raceTodo{ID: id, Title: "t"})
				}
			}
		}(w)
	}
	wg.Wait()
}

// TestMemoryRepository_UpdateDoesNotAliasCaller proves stored state is a copy.
func TestMemoryRepository_UpdateDoesNotAliasCaller(t *testing.T) {
	repo := NewMemoryRepo[raceTodo]()
	ctx := context.Background()

	entity := &raceTodo{ID: "x1", Title: "original"}
	if err := repo.Create(ctx, entity); err != nil {
		t.Fatal(err)
	}
	entity.Title = "mutated-after-create" // caller keeps mutating

	got, err := repo.FindByID(ctx, "x1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "original" {
		t.Fatalf("stored entity aliased the caller's pointer: %q", got.Title)
	}

	updated := &raceTodo{ID: "x1", Title: "v2"}
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatal(err)
	}
	updated.Title = "mutated-after-update"
	got, _ = repo.FindByID(ctx, "x1")
	if got.Title != "v2" {
		t.Fatalf("update aliased caller entity: %q", got.Title)
	}
}

// TestSafeMapRange_CallbackMayWrite is a deadlock regression test: fn used to
// run under RLock, so any write from inside the callback deadlocked.
func TestSafeMapRange_CallbackMayWrite(t *testing.T) {
	m := NewSafeMap[string, int]()
	for i := 0; i < 10; i++ {
		m.Set(string(rune('a'+i)), i)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Range(func(k string, v int) bool {
			m.Set("x"+k, v) // write inside callback
			if v == 5 {     // exercise early stop too
				return false
			}
			return true
		})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Range deadlocked when callback writes to the map")
	}
}

// TestSafeMap_ConcurrentRangeWriters hammers Range against writers under -race.
func TestSafeMap_ConcurrentRangeWriters(t *testing.T) {
	m := NewSafeMap[string, int]()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.Set(string(rune('a'+i%26))+string(rune('a'+i/26%26)), i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.Range(func(k string, v int) bool { return true })
		}
	}()
	wg.Wait()
}
