package analysis

import "sort"

const EvidenceVersion = 1

type Evidence struct {
	SchemaVersion int            `json:"schema_version"`
	Symbol        Symbol         `json:"symbol"`
	Analysis      StaticEvidence `json:"analysis"`
	Framework     string         `json:"framework"`
	DynamicRisk   bool           `json:"dynamic_risk"`
	Complete      bool           `json:"complete"`
	Warnings      []string       `json:"warnings"`
}
type StaticEvidence struct {
	PotentialExternalReachability bool        `json:"potential_external_reachability"`
	ReferencesTruncated           bool        `json:"references_truncated"`
	DirectReferenceCount          int         `json:"direct_reference_count"`
	Reachable                     bool        `json:"reachable_from_entrypoint"`
	References                    []Reference `json:"references"`
	Callers                       []string    `json:"callers"`
}

// BuildEvidence has stable ordering regardless of analyzer emission order.
func BuildEvidence(p *Project, symbol Symbol, reachable bool, refs []Reference) Evidence {
	count := len(refs)
	refs = append([]Reference{}, refs...)
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	truncated := len(refs) > 64
	if truncated {
		refs = refs[:64]
	}
	callers := []string{}
	seen := map[string]bool{}
	for _, r := range refs {
		if !seen[r.From] {
			callers = append(callers, r.From)
			seen[r.From] = true
		}
	}
	sort.Strings(callers)
	warnings := append([]string{}, p.Warnings...)
	sort.Strings(warnings)
	return Evidence{
		SchemaVersion: EvidenceVersion, Symbol: symbol,
		Analysis:  StaticEvidence{ReferencesTruncated: truncated, DirectReferenceCount: count, Reachable: reachable, References: refs, Callers: callers},
		Framework: p.Framework, DynamicRisk: p.DynamicRisk || symbol.DynamicRisk, Complete: p.Complete, Warnings: warnings,
	}
}
