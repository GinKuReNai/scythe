package scan

import (
	"sort"

	"github.com/GinKuReNai/scythe/internal/analysis"
)

type Candidate struct {
	Evidence analysis.Evidence `json:"evidence"`
	Reason   string            `json:"reason"`
}

func FindCandidates(p *analysis.Project) ([]Candidate, int) {
	reachable := analysis.Reachable(p.Roots, p.Edges)
	external := analysis.PotentialExternalReachability(p)
	refs := map[string][]analysis.Reference{}
	for _, ref := range p.Edges {
		refs[ref.To] = append(refs[ref.To], ref)
	}
	candidates := []Candidate{}
	live := 0
	for _, s := range p.Symbols {
		if reachable[s.ID] || s.Public || s.FrameworkEntry {
			live++
			continue
		}
		reason := "unreachable from known entry points"
		if len(refs[s.ID]) > 0 {
			reason = "only referenced within unreachable code"
		}
		evidence := analysis.BuildEvidence(p, s, false, refs[s.ID])
		evidence.Analysis.PotentialExternalReachability = external[s.ID]
		candidates = append(candidates, Candidate{evidence, reason})
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i].Evidence.Symbol, candidates[j].Evidence.Symbol
		if a.File != b.File {
			return a.File < b.File
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.ID < b.ID
	})
	return candidates, live
}
