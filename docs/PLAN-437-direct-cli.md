# Plan: Direct CLI execution — `skrptiq` as a unix tool (GH#437)

## Context

GH#437 asks for non-interactive CLI execution: run workflows, query data, and produce output without the TUI. The `scan` subcommand (GH#497, just shipped) established the pattern — headless subcommands dispatched in `main.go` before `tea.NewProgram`. This extends that to `run`, `list`, `show`, and `hub`.

## Approach

### New package: `internal/cli/`

All headless subcommand handlers. Each subcommand is a function that parses its own flags, calls engine functions, formats output, and returns an exit code. Same pattern as `internal/scan/`.

### Subcommands to implement

| Command | Purpose | Engine API |
|---------|---------|-----------|
| `skrptiq run <workflow> [--input k=v] [--output path] [--json] [--yes\|--strict]` | Execute workflow | `engine.RunWorkflow` |
| `skrptiq list <type> [--json] [-q]` | List nodes/profiles/runs | `engine.NodesByType`, `engine.Profiles`, `engine.ListExecutions` |
| `skrptiq show <type> <name\|id> [--json]` | Show node/run detail | `engine.FindNodeByTitle`, `engine.GetRunDetail` |
| `skrptiq hub <sub> [args]` | Hub operations | `engine.Hub.Search`, `engine.HubImports` |

### Files to create

| File | Purpose |
|------|---------|
| `internal/cli/run.go` | `skrptiq run` — workflow execution with streaming output |
| `internal/cli/list.go` | `skrptiq list` — nodes, profiles, runs |
| `internal/cli/show.go` | `skrptiq show` — node content, run detail |
| `internal/cli/hub.go` | `skrptiq hub` — search, list imports |
| `internal/cli/common.go` | Shared: engine open, output formatting, exit codes |
| `internal/cli/cli_test.go` | Tests |

### File to modify

| File | Change |
|------|--------|
| `main.go` | Add `run`, `list`, `show`, `hub` subcommand dispatch |

## Design decisions

**1. Shared engine open** — All subcommands need the engine DB. `common.go` provides `OpenEngine(dbPath)` that opens the engine with the same DefaultDBPath logic, returning an `*eng.App`. The `--db-path` global flag is passed through.

**2. Output modes** — Three modes matching the issue spec:
- Default: human-readable plain text to stdout
- `--json`: structured JSON to stdout
- `-q`: quiet — just IDs or minimal output (for scripting)

**3. `skrptiq run` streaming** — Workflow execution streams step progress to stderr (so stdout stays clean for output). Final result goes to stdout. `--output <path>` writes the final step's output to a file instead.

**4. Gate handling** — Four modes per the issue:
- Default: prompt on stdin (`Gate: <title>. Approve? [y/N]`)
- `--yes`: auto-approve all gates
- `--strict`: fail with exit 1 on any gate
- `--gate-timeout N`: wait N seconds, fail if no input

**5. stdin support** — `--input content=-` reads from stdin. Only one `-` input allowed.

**6. Exit codes** — Per the issue: 0 success, 1 workflow failed/gate rejected, 2 invalid args, 3 missing dependency, 4 timeout.

## Execution flow

### `skrptiq run "Blog Post Pipeline" --input topic="AI trends"`

```
1. Parse flags (--input, --output, --json, --yes/--strict/--gate-timeout)
2. Open engine (shared DB)
3. Find workflow by title (engine.FindNodeByTitle)
4. Build execution plan (engine.BuildPlan)
5. Resolve inputs: merge --input flags with stdin (if content=-)
6. Start execution with progress callback:
   - step-started → print to stderr
   - step-chunk → print to stderr (streaming)
   - step-completed → print to stderr
   - step-awaiting-input → handle gate per flag mode
   - step-failed → print to stderr
7. On completion: write final output to stdout (or --output file)
8. Return exit code
```

### `skrptiq list nodes --type workflow --json`

```
1. Parse flags (--type, --json, -q)
2. Open engine
3. Query: NodesByType or GetAllNodes
4. Format and print
5. Exit 0
```

### `skrptiq show run abc123 --step 2`

```
1. Parse args (type + identifier)
2. Open engine
3. Route: "node" → FindNodeByTitle, "run" → GetRunDetail
4. Format and print (with --step for run step output)
5. Exit 0
```

## main.go routing

```go
args := flag.Args()
if len(args) > 0 {
    switch args[0] {
    case "scan":
        // existing scan dispatch
    case "run":
        os.Exit(cli.Run(args[1:], *dbPath))
    case "list":
        os.Exit(cli.List(args[1:], *dbPath))
    case "show":
        os.Exit(cli.Show(args[1:], *dbPath))
    case "hub":
        os.Exit(cli.Hub(args[1:], *dbPath))
    case "version":
        fmt.Println("skrptiq " + version.Full())
        return
    }
}
```

No args → TUI (existing behaviour).

## Sequencing

1. `internal/cli/common.go` — engine open, output helpers, exit codes
2. `internal/cli/list.go` + tests — simplest, read-only queries
3. `internal/cli/show.go` + tests — read-only with formatting
4. `internal/cli/hub.go` + tests — read-only Hub queries
5. `internal/cli/run.go` + tests — most complex (execution, streaming, gates)
6. `main.go` routing update
7. Commit per logical unit

## Tests

- `TestList_Nodes` — list nodes with --json and -q modes
- `TestList_InvalidType` — bad type returns exit 2
- `TestShow_Node` — show node content
- `TestShow_NotFound` — missing node returns exit 2
- `TestRun_MissingWorkflow` — nonexistent workflow returns exit 2
- `TestRun_ValidWorkflow` — execute against test DB (needs workflow + skills loaded)
- Gate handling tests: --yes auto-approves, --strict fails

## Verification

1. `go test ./internal/cli/...` — all tests pass
2. `go build && ./skrptiq list nodes --type workflow --json` — outputs JSON
3. `./skrptiq show node "Blog Post Pipeline"` — shows content
4. `./skrptiq run "Blog Post Pipeline" --input topic="test" --yes` — executes
5. `go vet ./...` — clean

## What's NOT in scope

- `skrptiq login` — blocked on Hub auth (GH#440)
- Cron/scheduling — future feature
- `skrptiq publish` — Hub publishing, separate issue
