package sqladapter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rodrigocelsius/go-nitro/blinq/v2/field"
	sqladapter "github.com/rodrigocelsius/go-nitro/data/adapter/sql/v2"
	nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"
)

type txParent struct {
	nitro.Model
	Name string `db:"name"`
}

type txChild struct {
	nitro.Model
	ParentID nitro.UUID `db:"parent_id"`
	Label    string     `db:"label"`
}

func TestTx_CommitParentChild(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
CREATE TABLE parents (
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);
CREATE TABLE children (
  id TEXT PRIMARY KEY NOT NULL,
  parent_id TEXT NOT NULL,
  label TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);`
	if err := execRaw(ctx, db, schema); err != nil {
		t.Fatal(err)
	}

	var parentID nitro.UUID
	err = db.Tx(ctx, func(tx *sqladapter.DB) error {
		parents := sqladapter.StoreOf[txParent](tx, "parents")
		children := sqladapter.StoreOf[txChild](tx, "children")
		p, err := parents.CreateOne(ctx, &txParent{Name: "Ada"})
		if err != nil {
			return err
		}
		parentID = p.ID
		_, err = children.CreateOne(ctx, &txChild{ParentID: p.ID, Label: "note"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	parents := sqladapter.StoreOf[txParent](db, "parents")
	got, err := parents.Get(ctx, parentID)
	if err != nil || got.Name != "Ada" {
		t.Fatalf("parent: %#v err=%v", got, err)
	}
	children := sqladapter.StoreOf[txChild](db, "children")
	rows, err := children.Find(ctx, children.Query().Where(field.Col[any]("children", "parent_id").Eq(parentID)))
	if err != nil || len(rows) != 1 {
		t.Fatalf("children: %v len=%d", err, len(rows))
	}
}

func TestTx_Rollback(t *testing.T) {
	ctx := context.Background()
	db, err := sqladapter.OpenTest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := `CREATE TABLE items (
  id TEXT PRIMARY KEY NOT NULL,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);`
	if err := execRaw(ctx, db, schema); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("boom")
	err = db.Tx(ctx, func(tx *sqladapter.DB) error {
		store := sqladapter.StoreOf[txParent](tx, "items")
		if _, err := store.CreateOne(ctx, &txParent{Name: "x"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	store := sqladapter.StoreOf[txParent](db, "items")
	items, err := store.Find(ctx, store.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected rollback, got %d rows", len(items))
	}
}

func execRaw(ctx context.Context, db *sqladapter.DB, sql string) error {
	if std := db.StdDB(); std != nil {
		_, err := std.ExecContext(ctx, sql)
		return err
	}
	if pg := db.Pgx(); pg != nil {
		return pg.ExecSQL(ctx, sql)
	}
	return errors.New("no executor")
}
