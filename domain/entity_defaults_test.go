package domain_test

import (
	"testing"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

func TestApplyEntityDefaults_Create(t *testing.T) {
	type row struct {
		ID        string
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	var r row
	domain.ApplyEntityDefaults(&r, domain.EntityDefaults{AutoID: true, AutoTimestamps: true}, true)
	if r.ID == "" {
		t.Fatal("expected UUID")
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps")
	}
}

func TestApplyEntityDefaults_UpdateTouchesUpdatedAt(t *testing.T) {
	type row struct {
		ID        string
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	created := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	r := row{ID: "x", CreatedAt: created, UpdatedAt: created}
	domain.ApplyEntityDefaults(&r, domain.EntityDefaults{AutoTimestamps: true}, false)
	if !r.CreatedAt.Equal(created) {
		t.Fatal("CreatedAt should be preserved")
	}
	if !r.UpdatedAt.After(created) {
		t.Fatal("UpdatedAt should be refreshed")
	}
}
