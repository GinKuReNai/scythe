package scan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/GinKuReNai/scythe/internal/cache/sqlite"
	"github.com/GinKuReNai/scythe/internal/decision/jev"
	"github.com/GinKuReNai/scythe/internal/policy"
	"github.com/GinKuReNai/scythe/internal/typescript"
)

// This tests the full compiler -> HTTP -> SQLite -> policy vertical slice
// without sending fixture source to an external service.
func TestCompilerJevSQLiteVerticalSlice(t *testing.T) {
	var calls atomic.Int32
	response := `{"id":"dec_integration","model":"test","model_version":"test-v1","answers":{"runtime_reachable":{"type":"noul","noul":0.01},"framework_required":{"type":"noul","noul":0.01},"side_effect_risk":{"type":"noul","noul":0.01},"deletion_safety":{"type":"score","score":2.95,"probabilities":{"0":0,"1":0,"2":0.05,"3":0.95}},"action":{"type":"choice","choice":"delete","probabilities":{"delete":0.99,"keep":0,"review":0.01},"confidence":0.99}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, response) }))
	defer server.Close()
	script, _ := filepath.Abs("../../analyzer/typescript/dist/index.js")
	root, _ := filepath.Abs("../../testdata/basic")
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(&typescript.Analyzer{Script: script}, jev.NewClient(server.Client(), server.URL, "test", "fixture-key"), store, policy.Defaults(), "test", jev.QuestionSetVersion, 2)
	first, err := service.Scan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 5 || first.Statistics.SafeToDelete != 5 || first.Statistics.Reachable != 1 {
		t.Fatalf("first scan: %+v calls=%d", first.Statistics, calls.Load())
	}
	second, err := service.Scan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 5 || second.Statistics.CacheHits != 5 || second.Statistics.ModelCalls != 0 {
		t.Fatalf("second scan: %+v calls=%d", second.Statistics, calls.Load())
	}
}
