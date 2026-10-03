# Scythe

Find dead TypeScript code with compiler evidence, focused Jev judgments, and conservative policy.

Scythe's `deadcode` CLI builds a symbol graph before asking AI any questions. Traditional unused-code tools struggle with implicit framework entry points, public exports, dynamic behavior, and side effects. General coding agents are costly and unconstrained when asked to inspect an entire repository. Scythe collects focused evidence for each ambiguous candidate instead.

```text
TypeScript → Compiler API → Symbol / Reference Graph → Deterministic Rules
                                                         ↓
                                                      Evidence
                                                         ↓
                                                        Jev
                                                         ↓
                                                       Policy
                                                         ↓
                                              KEEP / REVIEW / SAFE TO DELETE
```

## Status and safety

Experimental TypeScript MVP under active development. No source files are modified. A result of **SAFE TO DELETE** is a recommendation, not an automatic edit or a correctness guarantee. Compiler reachability comes first; Jev evaluates focused evidence; deterministic policy sets the risk tolerance. Dynamic usage, side effects, public surfaces, incomplete analysis, and uncertainty prevent safe-delete recommendations.

Without `JEV_API_KEY`, ambiguous candidates are reported as REVIEW. Source excerpts and graph evidence for eligible candidates are sent to Jev when a key is configured. Use `--offline` to keep all analysis local.

## Installation from source

Requirements: Go 1.22+, Node.js 20+, npm. Use Go 1.26+ on current macOS releases; older Go linkers may produce binaries rejected by the OS.

```bash
git clone https://github.com/GinKuReNai/scythe.git
cd scythe
make build
./bin/deadcode version
```

Keep the built analyzer beside the binary (`bin/analyzer/`). Release packages and package-manager installation are planned.

## Quick start

```bash
./bin/deadcode scan ./testdata/basic --offline
export JEV_API_KEY='your-key'
./bin/deadcode scan /path/to/project
./bin/deadcode scan /path/to/project --format=json > result.json
./bin/deadcode explain deadFunction --path ./testdata/basic --offline
```

## Commands

- `deadcode scan [path]`: analyze a TypeScript project (default: current directory).
- `deadcode explain <symbol> --path <project>`: analyze the project and evaluate only matching symbols, showing their complete evidence. Use the symbol ID to disambiguate names.
- `deadcode cache clean`: clear cached model decisions.
- `deadcode version`: show the CLI version.

Use `--help` for flags. Logs and errors go to stderr; results go to stdout. `--offline` never calls Jev. `--no-cache` disables decision caching. `--analyzer` overrides the analyzer script path.

## Configuration

Optional `.deadcode.yaml` is read from the analyzed project directory. Precedence: CLI flags, environment, YAML, defaults. Unknown configuration fields are errors.

```yaml
version: 1
analysis:
  exclude:
    - "**/*.generated.ts"
    - "**/node_modules/**"
jev:
  model: jev-1.13
  concurrency: 8
  timeout_seconds: 30
policy:
  auto_delete_probability: 0.98
  max_runtime_reachability: 0.05
  max_framework_required_probability: 0.05
  max_side_effect_risk: 0.05
  min_deletion_safety: 2.9
cache:
  enabled: true
```

Environment: `JEV_API_KEY`, `DEADCODE_MODEL`, `DEADCODE_CONCURRENCY`, `DEADCODE_FORMAT`, `DEADCODE_ANALYZER`, `DEADCODE_CACHE_DIR`. Credentials are accepted only from the environment. Cache defaults to `<os.UserCacheDir()>/deadcode/cache.db`. `--offline` may reuse cached decisions; combine it with `--no-cache` for static evidence only. Cache entries have no TTL: use a pinned model for reproducibility and `cache clean` to refresh rolling aliases such as `jev-latest`.

## Output

Text reports include scanned files/symbols, candidate totals, policy outcomes, reasons, references, risk flags, and model probabilities when available. JSON has `schema_version: 1`, project metadata, statistics, and findings with deterministic evidence and optional decision metadata. For example, an offline candidate includes:

```text
src/index.ts:1  deadFunction
  Decision: REVIEW
  Reason: unreachable from known entry points
  References: 0; exported: false; side effects: false
  Policy: no model decision available
```

`explain <symbol> --format=json` includes the evidence used by the model. Reports contain source excerpts; treat them as project data.

## Architecture

The Go `scan.Service` orchestrates analysis, candidate extraction, evidence, bounded model calls, caching, and policy. The Node analyzer uses the TypeScript Compiler API and streams a versioned NDJSON protocol. Small interfaces isolate the analyzer, decision engine, cache, and reporters. SQLite is a disposable decision cache, not a domain database. Cache keys include evidence, model, and question/decision schema versions.

The analyzer tracks functions, classes, and variables, including references across files. Roots include module execution, package entry surfaces, executable entries, and conservative Next.js/Vite conventions. Unreachable subgraphs remain candidates even when their members reference one another.

## Current limitations

- TypeScript only; one tsconfig per scan. Project-reference/workspace traversal is planned.
- Dynamic imports, reflection-like usage, unknown external callers, class initialization, decorators, and framework conventions require conservative handling.
- Exports with unknown consumers and their transitive dependencies remain REVIEW.
- Framework support is heuristic (Node, Next.js, Vite), not a proof of complete runtime reachability.
- Declaration files, dependencies, and generated files are excluded. Excluding runtime code reduces completeness and prevents safe-delete recommendations.
- Only functions, classes, and variables are reported. Methods, types, interfaces, enums, and dependency cleanup are planned.
- No automatic deletion. SAFE TO DELETE remains experimental and requires human verification.

## Development and contributing

```bash
make build       # install/build analyzer, build distributable CLI layout
make test        # Go tests and analyzer fixture tests
make vet         # go vet
make format      # gofmt and Prettier
make check       # build, tests, vet, formatting check
```

Add regression fixtures for any reachability or safety change. Avoid expanding model input unnecessarily. Tests use fake HTTP responses and real SQLite, never the paid Jev API. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Roadmap

Safer mechanical fixes; SARIF and GitHub annotations; more framework conventions; workspace analysis; types/interfaces; dependency and deprecated-code cleanup; feature-flag cleanup; additional language adapters.

## Security

Never commit `JEV_API_KEY`. Keys are not stored in the cache, reports, or logs. The HTTP contract follows the [Jev decision API documentation](https://jev-ai.org/docs/decisions/). Only candidate evidence is sent to Jev. See [SECURITY.md](SECURITY.md) for reporting vulnerabilities.

## License

[MIT](LICENSE), copyright Akito Koga.
