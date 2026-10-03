package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/GinKuReNai/scythe/internal/scan"
)

type Reporter interface {
	Write(io.Writer, *scan.Result) error
}
type JSON struct{}

func (JSON) Write(w io.Writer, r *scan.Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

type Text struct{ Explain bool }

func (t Text) Write(w io.Writer, r *scan.Result) error {
	// Build once so output errors are propagated, including broken pipes.
	var b buffer
	s := r.Statistics
	b.printf("Deadcode Scan\n\nScanned: %d files, %d symbols\nReachable/preserved: %d\nCandidates: %d\nSafe to delete: %d\nNeeds review: %d\nKept: %d\nCache hits: %d; model calls: %d\n", s.Files, s.Symbols, s.Reachable, s.Candidates, s.SafeToDelete, s.Review, s.Kept, s.CacheHits, s.ModelCalls)
	for _, warning := range r.Project.Warnings {
		b.printf("Warning: %s\n", warning)
	}
	for _, f := range r.Findings {
		symbol := f.Evidence.Symbol
		b.printf("\n%s:%d  %s\n  ID: %s\n  Decision: %s\n  Reason: %s\n  References: %d; exported: %t; side effects: %t\n  Framework: %s; entrypoint: %t; public: %t\n  Dynamic risk: %t; analysis complete: %t\n  Policy: %s\n", symbol.File, symbol.StartLine, symbol.Name, symbol.ID, f.Outcome, f.Reason, f.Evidence.Analysis.DirectReferenceCount, symbol.Exported, symbol.SideEffects, f.Evidence.Framework, symbol.FrameworkEntry, symbol.Public, f.Evidence.DynamicRisk, f.Evidence.Complete, f.PolicyReason)
		if f.Evidence.Analysis.PotentialExternalReachability {
			b.printf("  External reachability: possible through exported symbols\n")
		}
		if d := f.Decision; d != nil {
			b.printf("  Jev: delete %.1f%%; review %.1f%%; keep %.1f%% (cached: %t)\n  Runtime reachable: %.1f%%; framework required: %.1f%%; side-effect risk: %.1f%%\n  Deletion safety: %.3f / 3; model: %s; version: %s; decision: %s\n", 100*d.DeleteProbability, 100*d.ReviewProbability, 100*d.KeepProbability, f.Cached, 100*d.RuntimeReachability, 100*d.FrameworkRequired, 100*d.SideEffectRisk, d.DeletionSafety, d.Model, d.ModelVersion, d.DecisionID)
		}
		if t.Explain {
			data, err := json.MarshalIndent(f.Evidence, "", "  ")
			if err != nil {
				return err
			}
			b.printf("  Evidence:\n%s\n", data)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

type buffer struct{ data []byte }

func (b *buffer) printf(format string, args ...any) { b.data = fmt.Appendf(b.data, format, args...) }
func (b *buffer) String() string                    { return string(b.data) }
