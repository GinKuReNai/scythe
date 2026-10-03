package analysis

import (
	"reflect"
	"testing"
)

func TestReachable(t *testing.T) {
	cases := []struct {
		name  string
		roots []string
		edges []Reference
		want  map[string]bool
	}{
		{"empty", nil, nil, map[string]bool{}},
		{"cycles", []string{"entry"}, []Reference{{From: "entry", To: "a"}, {From: "a", To: "b"}, {From: "b", To: "a"}, {From: "deadA", To: "deadB"}}, map[string]bool{"entry": true, "a": true, "b": true}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reachable(tt.roots, tt.edges); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
func TestEvidenceOrdering(t *testing.T) {
	p := &Project{Complete: true, Warnings: []string{"z", "a"}}
	refs := []Reference{{From: "z", To: "x", File: "a.ts", Line: 2}, {From: "a", To: "x", File: "a.ts", Line: 1}}
	a := BuildEvidence(p, Symbol{ID: "x"}, false, refs)
	b := BuildEvidence(p, Symbol{ID: "x"}, false, []Reference{refs[1], refs[0]})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("evidence ordering changed")
	}
	if refs[0].From != "z" {
		t.Fatal("mutated caller references")
	}
}
