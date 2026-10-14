package uuid_test

import (
	"encoding/json"
	"testing"

	"github.com/rodrigocelsius/go-nitro/pkg/uuid/v2"
)

func TestUUID_ScanValueJSON(t *testing.T) {
	u := uuid.New()
	v, err := u.Value()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := v.(string)
	if !ok || s != u.String() {
		t.Fatalf("Value: %v", v)
	}
	var u2 uuid.UUID
	if err := u2.Scan(s); err != nil {
		t.Fatal(err)
	}
	if u2 != u {
		t.Fatal("scan mismatch")
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var u3 uuid.UUID
	if err := json.Unmarshal(b, &u3); err != nil {
		t.Fatal(err)
	}
	if u3 != u {
		t.Fatal("json mismatch")
	}
}
