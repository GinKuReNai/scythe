package typescript

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/GinKuReNai/scythe/internal/analysis"
)

type Analyzer struct {
	Script  string
	Exclude []string
	Secret  string
}

type record struct {
	Version int             `json:"protocolVersion"`
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
}

// Analyze uses one cancellable subprocess per project and validates its stream.
func (a *Analyzer) Analyze(ctx context.Context, root string) (*analysis.Project, error) {
	request := struct {
		Version int      `json:"protocolVersion"`
		Method  string   `json:"method"`
		Root    string   `json:"root"`
		Exclude []string `json:"exclude"`
	}{1, "analyze", root, append([]string{}, a.Exclude...)}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "node", a.Script)
	cmd.WaitDelay = 2 * time.Second
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "JEV_API_KEY=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Stdin = bytes.NewReader(append(input, '\n'))
	stderr := &limitedBuffer{limit: 8192}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open analyzer output: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("TypeScript analyzer executable could not start (Node.js and built analyzer required): %w", err)
	}
	p, parseErr := decode(stdout)
	if parseErr != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if parseErr != nil {
		parseErr = errors.New(redact(parseErr.Error(), a.Secret))
		return nil, fmt.Errorf("invalid analyzer response: %w; %s", parseErr, redact(stderr.String(), a.Secret))
	}
	if waitErr != nil {
		return nil, fmt.Errorf("TypeScript analyzer failed: %w; %s", waitErr, redact(stderr.String(), a.Secret))
	}
	if a.Secret != "" {
		data, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(data), a.Secret) {
			return nil, errors.New("analyzer metadata contains credentials")
		}
		for i := range p.Symbols {
			symbol := &p.Symbols[i]
			for _, value := range []string{symbol.ID, symbol.Name, symbol.File} {
				if strings.Contains(value, a.Secret) {
					return nil, errors.New("analyzer symbol metadata contains credentials")
				}
			}
			symbol.Source = redact(symbol.Source, a.Secret)
		}
		for _, ref := range p.Edges {
			for _, value := range []string{ref.From, ref.To, ref.File} {
				if strings.Contains(value, a.Secret) {
					return nil, errors.New("analyzer reference metadata contains credentials")
				}
			}
		}
		for _, root := range p.Roots {
			if strings.Contains(root, a.Secret) {
				return nil, errors.New("analyzer root metadata contains credentials")
			}
		}
	}
	return p, nil
}

func decode(reader io.Reader) (*analysis.Project, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	p := &analysis.Project{}
	seenProject, done := false, false
	ids := map[string]bool{}
	for scanner.Scan() {
		if done {
			return nil, errors.New("records after done")
		}
		var r record
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			return nil, err
		}
		if r.Version != 1 {
			return nil, fmt.Errorf("unsupported protocol version %d", r.Version)
		}
		if len(r.Data) == 0 || string(r.Data) == "null" {
			return nil, errors.New("missing record data")
		}
		if !seenProject && r.Type != "project" {
			return nil, errors.New("project must be the first record")
		}
		switch r.Type {
		case "project":
			if err := requireFields(r.Data, "root_dir", "tsconfig_path", "framework", "files", "complete", "warnings", "dynamic_risk"); err != nil {
				return nil, err
			}
			if seenProject {
				return nil, errors.New("duplicate project")
			}
			seenProject = true
			if err := json.Unmarshal(r.Data, p); err != nil {
				return nil, err
			}
			if p.RootDir == "" || p.TSConfigPath == "" || p.Files < 0 {
				return nil, errors.New("invalid project metadata")
			}
			if p.Framework != "unknown" && p.Framework != "node" && p.Framework != "nextjs" && p.Framework != "vite" {
				return nil, errors.New("invalid framework")
			}
		case "symbol":
			if err := requireFields(r.Data, "id", "name", "kind", "file", "start_line", "end_line", "exported", "public", "framework_entry", "side_effects", "dynamic_risk", "source", "source_truncated"); err != nil {
				return nil, err
			}
			var s analysis.Symbol
			if err := json.Unmarshal(r.Data, &s); err != nil {
				return nil, err
			}
			if s.ID == "" || s.Name == "" || s.File == "" || s.StartLine < 1 || s.EndLine < s.StartLine || ids[s.ID] || !(s.Kind == "function" || s.Kind == "class" || s.Kind == "variable") {
				return nil, errors.New("invalid or duplicate symbol")
			}
			ids[s.ID] = true
			p.Symbols = append(p.Symbols, s)
		case "edge":
			if err := requireFields(r.Data, "from", "to", "file", "line"); err != nil {
				return nil, err
			}
			var e analysis.Reference
			if err := json.Unmarshal(r.Data, &e); err != nil {
				return nil, err
			}
			if e.From == "" || e.To == "" || e.File == "" || e.Line < 1 {
				return nil, errors.New("invalid reference")
			}
			p.Edges = append(p.Edges, e)
		case "root":
			if err := requireFields(r.Data, "id"); err != nil {
				return nil, err
			}
			var root struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(r.Data, &root); err != nil {
				return nil, err
			}
			if root.ID == "" {
				return nil, errors.New("empty root")
			}
			p.Roots = append(p.Roots, root.ID)
		case "done":
			done = true
		default:
			return nil, fmt.Errorf("unknown record type %q", r.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !done {
		return nil, errors.New("missing done record")
	}
	for _, e := range p.Edges {
		if !ids[e.To] || (!ids[e.From] && !strings.HasPrefix(e.From, "module:")) {
			return nil, errors.New("reference to unknown node")
		}
	}
	for _, id := range p.Roots {
		if !ids[id] && !strings.HasPrefix(id, "module:") {
			return nil, errors.New("unknown root")
		}
	}
	return p, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

func requireFields(data json.RawMessage, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			return fmt.Errorf("missing or null analyzer field %s", name)
		}
	}
	return nil
}

func redact(text, secret string) string {
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "[REDACTED]")
}
