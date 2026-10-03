package analysis

// Reachable traverses references, including synthetic module-execution nodes.
func Reachable(roots []string, edges []Reference) map[string]bool {
	adjacency := make(map[string][]string)
	for _, e := range edges {
		adjacency[e.From] = append(adjacency[e.From], e.To)
	}
	seen := make(map[string]bool)
	stack := append([]string(nil), roots...)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, adjacency[id]...)
	}
	return seen
}

// PotentialExternalReachability protects dependencies of exports whose callers
// may live outside the scanned project, including namespace-import consumers.
func PotentialExternalReachability(p *Project) map[string]bool {
	roots := []string{}
	for _, s := range p.Symbols {
		if s.Exported {
			roots = append(roots, s.ID)
		}
	}
	return Reachable(roots, p.Edges)
}
