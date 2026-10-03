package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/GinKuReNai/scythe/internal/decision"
)

func TestPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cache.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, hit, err := db.Get(ctx, "key"); err != nil || hit {
		t.Fatalf("miss: %v %v", hit, err)
	}
	r := decision.Result{Action: "review", ReviewProbability: 1, Model: "test", ModelVersion: "v1", DecisionID: "id", RawResponse: []byte(`{"id":"id"}`)}
	if err = db.Put(ctx, "key", r); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, hit, err := db.Get(ctx, "key")
	if err != nil || !hit || got.DecisionID != "id" || string(got.RawResponse) != string(r.RawResponse) {
		t.Fatalf("persistent hit: %+v %t %v", got, hit, err)
	}
	r.DecisionID = "updated"
	if err = db.Put(ctx, "key", r); err != nil {
		t.Fatal(err)
	}
	got, _, _ = db.Get(ctx, "key")
	if got.DecisionID != "updated" {
		t.Fatal("update failed")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err = db.Get(canceled, "key"); err == nil {
		t.Fatal("canceled read succeeded")
	}
	if err = db.Clean(ctx); err != nil {
		t.Fatal(err)
	}
	_, hit, err = db.Get(ctx, "key")
	if err != nil || hit {
		t.Fatal("clean failed")
	}
}
