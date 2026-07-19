package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skrptiq/skrptiq-cli/internal/app"
	"github.com/skrptiq/skrptiq-cli/internal/bridge"
	"github.com/skrptiq/skrptiq-cli/internal/cli"
	"github.com/skrptiq/skrptiq-cli/internal/scan"
	"github.com/skrptiq/skrptiq-cli/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

func printHelp() {
	fmt.Print(`skrptiq — personalised AI agents in your terminal

Usage:
  skrptiq                     Launch interactive session
  skrptiq <command> [args]    Run a command directly

Commands:
  run <workflow> [flags]      Execute a workflow
  list [type] [flags]         List nodes (workflows, skills, prompts, ...)
  show <name> [flags]         Display node content and metadata
  hub <subcommand>            Browse and import community skrpts
  scan [--no-resolve-deps] <path>
                              Parse and validate a directory
  new <name>                  Create a new skrpt directory
  lint <dir> [--auto-fix]     Check for identity/manifest issues
  migrate-identity <dir>      Rewrite legacy manifest.id to canonical form
  catalogue <dir>             List all skrpts in a directory (JSON)
  sign <dir> --key-env <VAR>  Sign a bundle (adds integrity + trust blocks)
  verify <bundle.zip>         Verify a signed bundle
  bridge <status|enable|disable>
                              Manage the browser native-messaging bridge
  builtins [--json]           List the engine's built-in operations
  help                        Show this help message
  version                     Print version and exit

Run flags:
  --input key=value           Set input variable (repeatable; value=- reads stdin)
  --output <path>             Write final output to a file
  --json                      Emit structured JSON output
  --yes                       Auto-approve all gate steps
  --strict                    Fail on any gate rejection
  --gate-timeout <seconds>    Auto-approve gates after timeout

List/show flags:
  --type <nodeType>           Filter by node type
  --tag <name>                Filter by tag
  --json                      Emit structured JSON output
  -q                          Quiet mode — IDs only

Hub subcommands:
  hub list                    List imported skrpts
  hub search <query>          Search community skrpts
  hub import <slug>           Import a skrpt from Hub
  hub update                  Check for updates to imported skrpts

Global flags:
  --db-path <path>            Path to SQLite database (overrides default)
  --version                   Print version and exit

Interactive session:
  Type naturally to chat with your AI team, or use /commands.
  Run /help inside the session for interactive command reference.

Examples:
  skrptiq                             Start interactive session
  skrptiq run "Blog Post Pipeline"    Execute a workflow by name
  skrptiq list workflows              List all workflows
  skrptiq hub search "blog"           Search community skrpts
  skrptiq new my-skrpt                Create a new skrpt directory
  skrptiq lint ./my-skrpt --auto-fix  Fix identity issues in place
  echo "draft" | skrptiq run "Edit"   Pipe input to a workflow
`)
}

func main() {
	// Hidden native-messaging bridge modes (GH#866) — spawned by the engine
	// (--mode mcp) or by Chrome via the native-messaging wrapper (--mode host),
	// never by a user. Handle before any flag parsing or engine init.
	if handled, code := bridge.Dispatch(os.Args[1:]); handled {
		os.Exit(code)
	}

	dbPath := flag.String("db-path", "", "Path to SQLite database (overrides default)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Usage = func() { printHelp() }
	flag.Parse()

	if *showVersion {
		fmt.Println("skrptiq " + version.Full())
		return
	}

	// Subcommand dispatch — headless commands run before TUI.
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h":
			printHelp()
			return
		case "scan":
			scanFlags := flag.NewFlagSet("scan", flag.ExitOnError)
			jsonOutput := scanFlags.Bool("json", false, "Output as JSON")
			noResolveDeps := scanFlags.Bool("no-resolve-deps", false,
				"Skip Hub-fetching of declared dependency node lists (local-dev / offline only; weaker than the publish gate)")
			scanFlags.Parse(args[1:])
			scanPath := scanFlags.Arg(0)
			if scanPath == "" {
				fmt.Fprintln(os.Stderr, "Usage: skrptiq scan [--json] [--no-resolve-deps] <path>")
				os.Exit(2)
			}
			os.Exit(scan.RunWithOptions(scanPath, *jsonOutput, scan.Options{
				NoResolveDeps: *noResolveDeps,
				HubBaseURL:    os.Getenv("SKRPTIQ_HUB_URL"),
			}, os.Stdout))
		case "run":
			os.Exit(cli.Run(args[1:], *dbPath))
		case "list":
			os.Exit(cli.List(args[1:], *dbPath))
		case "show":
			os.Exit(cli.Show(args[1:], *dbPath))
		case "hub":
			os.Exit(cli.Hub(args[1:], *dbPath))
		case "new":
			os.Exit(cli.New(args[1:]))
		case "lint":
			os.Exit(cli.Lint(args[1:]))
		case "migrate-identity":
			os.Exit(cli.MigrateIdentity(args[1:]))
		case "catalogue":
			os.Exit(cli.Catalogue(args[1:]))
		case "sign":
			os.Exit(cli.Sign(args[1:]))
		case "verify":
			os.Exit(cli.Verify(args[1:]))
		case "bridge":
			os.Exit(cli.Bridge(args[1:], *dbPath))
		case "builtins":
			os.Exit(cli.Builtins(args[1:]))
		case "version":
			fmt.Println("skrptiq " + version.Full())
			return
		}
	}

	model, err := app.New(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(&model, tea.WithInputTTY())
	model.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: terminal session failed\n\n"+
			"  This usually means the terminal does not support interactive input.\n"+
			"  Try: run from a standard terminal emulator (not a pipe or script).\n\n"+
			"  Detail: %v\n", err)
		os.Exit(1)
	}
}
