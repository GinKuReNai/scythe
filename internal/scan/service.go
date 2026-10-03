package scan

import (
	"context"
	"fmt"

	"github.com/GinKuReNai/scythe/internal/analysis"
	"github.com/GinKuReNai/scythe/internal/cache"
	"github.com/GinKuReNai/scythe/internal/decision"
	"github.com/GinKuReNai/scythe/internal/policy"
	"golang.org/x/sync/errgroup"
)

type Analyzer interface {
	Analyze(context.Context, string) (*analysis.Project, error)
}
type DecisionEngine interface {
	Decide(context.Context, analysis.Evidence) (decision.Result, error)
}
type Cache interface {
	Get(context.Context, string) (decision.Result, bool, error)
	Put(context.Context, string, decision.Result) error
}
type Service struct {
	analyzer         Analyzer
	decider          DecisionEngine
	cache            Cache
	policy           policy.Config
	model, questions string
	concurrency      int
}

func NewService(analyzer Analyzer, decider DecisionEngine, store Cache, thresholds policy.Config, model, questions string, concurrency int) *Service {
	return &Service{analyzer, decider, store, thresholds, model, questions, concurrency}
}

func (s *Service) Scan(ctx context.Context, path string) (*Result, error) {
	return s.scan(ctx, path, "")
}

// Explain evaluates only matching symbols; unrelated candidates are not sent to the model.
func (s *Service) Explain(ctx context.Context, path, symbol string) (*Result, error) {
	return s.scan(ctx, path, symbol)
}
func (s *Service) scan(ctx context.Context, path, symbol string) (*Result, error) {
	if s.concurrency < 1 {
		return nil, fmt.Errorf("decision concurrency must be positive")
	}
	p, err := s.analyzer.Analyze(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("analyze TypeScript project: %w", err)
	}
	candidates, live := FindCandidates(p)
	if symbol != "" {
		selected := []Candidate{}
		reachable := analysis.Reachable(p.Roots, p.Edges)
		external := analysis.PotentialExternalReachability(p)
		for _, sym := range p.Symbols {
			if sym.Name != symbol && sym.ID != symbol {
				continue
			}
			refs := []analysis.Reference{}
			for _, ref := range p.Edges {
				if ref.To == sym.ID {
					refs = append(refs, ref)
				}
			}
			reason := "unreachable from known entry points"
			if reachable[sym.ID] {
				reason = "reachable from known entry points"
			}
			evidence := analysis.BuildEvidence(p, sym, reachable[sym.ID], refs)
			evidence.Analysis.PotentialExternalReachability = external[sym.ID]
			selected = append(selected, Candidate{evidence, reason})
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("symbol %q not found; use a symbol ID to disambiguate names", symbol)
		}
		candidates = selected
	}
	result := &Result{SchemaVersion: 1, Project: *p, Findings: make([]Finding, len(candidates)), Statistics: Statistics{Files: p.Files, Symbols: len(p.Symbols), Reachable: live, Candidates: len(candidates)}}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.concurrency)
	for i, candidate := range candidates {
		if err := groupCtx.Err(); err != nil {
			break
		}
		i, candidate := i, candidate
		group.Go(func() error {
			finding := Finding{Candidate: candidate}
			if outcome, reason := policy.StaticOutcome(candidate.Evidence); outcome != "" {
				finding.Outcome = outcome
				finding.PolicyReason = reason
				result.Findings[i] = finding
				return nil
			}
			key, err := cache.Key(candidate.Evidence, s.model, s.questions)
			if err != nil {
				return err
			}
			if s.cache != nil {
				cached, hit, err := s.cache.Get(groupCtx, key)
				if err != nil {
					return err
				}
				if hit {
					finding.Decision = &cached
					finding.Cached = true
				}
			}
			if finding.Decision == nil && s.decider != nil {
				decided, err := s.decider.Decide(groupCtx, candidate.Evidence)
				if err != nil {
					return fmt.Errorf("decide %s: %w", candidate.Evidence.Symbol.ID, err)
				}
				if err = decided.Validate(); err != nil {
					return fmt.Errorf("invalid decision for %s: %w", candidate.Evidence.Symbol.ID, err)
				}
				finding.Decision = &decided
				if s.cache != nil {
					if err = s.cache.Put(groupCtx, key, decided); err != nil {
						return err
					}
				}
			}
			finding.Outcome, finding.PolicyReason = s.policy.Apply(candidate.Evidence, finding.Decision)
			result.Findings[i] = finding
			return nil
		})
	}
	if err = group.Wait(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	for _, finding := range result.Findings {
		switch finding.Outcome {
		case policy.SafeToDelete:
			result.Statistics.SafeToDelete++
		case policy.Keep:
			result.Statistics.Kept++
		default:
			result.Statistics.Review++
		}
		if finding.Cached {
			result.Statistics.CacheHits++
		} else if finding.Decision != nil {
			result.Statistics.ModelCalls++
		}
	}
	return result, nil
}
