package cache

import (
	"testing"

	"github.com/GinKuReNai/scythe/internal/analysis"
)

func TestKey(t *testing.T) {
	e := analysis.BuildEvidence(&analysis.Project{Complete: true}, analysis.Symbol{ID: "id", Source: "function f() {}"}, false, nil)
	first, err := Key(e, "model", "q1")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Key(e, "model", "q1")
	if first != second {
		t.Fatal("unstable key")
	}
	for _, tt := range []struct{ model, questions string }{{"new-model", "q1"}, {"model", "q2"}} {
		key, _ := Key(e, tt.model, tt.questions)
		if key == first {
			t.Fatal("version did not invalidate key")
		}
	}
	e.Symbol.Source = "function f() { return 1; }"
	key, _ := Key(e, "model", "q1")
	if key == first {
		t.Fatal("source did not invalidate key")
	}
}
