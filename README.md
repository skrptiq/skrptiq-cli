# skrptiq

Personalised AI agents in your terminal. A single Go binary that gives you the
same execution engine as the skrptiq desktop app — workflows, profiles, MCP
tools — with a readline REPL and Unix pipe support.

## Quick Start

```sh
# Install
curl -fsSL https://skrptiq.com/install.sh | sh

# Launch the interactive session
skrptiq

# Or run commands directly
skrptiq list workflows            # See what's available
skrptiq run "Blog Post Pipeline"  # Execute a workflow
skrptiq hub search "blog"         # Browse community skrpts
```

Inside the interactive session, type naturally to chat with your AI team or use
`/commands`:

```
> Write a product description for the new feature
> /run Blog Post Pipeline
> /hub search seo
> /help
```

## Install

**curl (macOS / Linux):**

```sh
curl -fsSL https://skrptiq.com/install.sh | sh
```

**Go install:**

```sh
go install github.com/skrptiq/skrptiq-cli@latest
```

**From source:**

```sh
git clone https://github.com/skrptiq/skrptiq-cli.git
cd skrptiq-cli
go build -o skrptiq .
```

## Usage

```
skrptiq                     Launch interactive session
skrptiq <command> [args]    Run a command directly
```

### Commands

| Command | Description |
|---------|-------------|
| `run <workflow>` | Execute a workflow (`--input k=v`, `--json`, `--yes`) |
| `list [type]` | List nodes — workflows, skills, prompts, etc. (`--tag`, `--json`) |
| `show <name>` | Display node content and metadata |
| `hub list\|search\|import\|update` | Browse and import community skrpts |
| `scan <path>` | Parse and validate a directory |
| `help` | Show help |
| `version` | Print version |

### Pipe support

skrptiq reads stdin when a workflow input uses `value=-`:

```sh
echo "rough draft" | skrptiq run "Edit" --input content=-
cat notes.md | skrptiq run "Summarise" --input text=-
```

### Interactive commands

Once inside the session, run `/help` for the full command reference. Key
commands:

- `/chat` — talk to your AI team
- `/run <name>` — execute a workflow
- `/list`, `/show`, `/search` — browse your library
- `/hub search <query>` — find community skrpts
- `/profile use <name>` — switch voice profile
- `/dials set <dial> <value>` — adjust persona

## Requirements

- macOS, Linux, or Windows
- Shared database with the skrptiq desktop app (`~/Library/Application Support/skrptiq/data/skrptiq.db`)
- At least one AI provider configured (Anthropic, OpenAI, or Gemini)

## Licence

Proprietary. See [skrptiq.com](https://skrptiq.com) for terms.
