# go-minolas

A collection of reusable Go utilities for common tasks in daily development work.

## Development Workflow

### Agent Integration Workflow
When working with AI coding agents:
- Contractual design, functionality, and security documentation is maintained privately by the maintainer outside this public repository; the repository itself stays self-contained.
- Private workspace context, agent commands, or custom workspace settings must never be committed to this public submodule repository.

## Installation

```bash
go get github.com/tnotstar/go-minolas
```

## Packages

### `pkg/cli`

Command-line interface utilities for building interactive CLI applications.

**Features:**
- User input reading with validation
- Confirmation prompts
- Option selection menus

**Example:**
```go
import "github.com/tnotstar/go-minolas/pkg/cli"

// Get user confirmation
if cli.Confirm("Do you want to continue?") {
    // proceed
}

// Read user input with validation
name, err := cli.ReadInput("Enter your name: ", func(s string) error {
    if s == "" {
        return errors.New("name cannot be empty")
    }
})
```

### `pkg/cli/argparse`

A lightweight, Python-inspired argument parsing library utilizing pure Go standard library functionality. It provides type-safe functional configuration, automatic usage generation, and nestable subcommand parsing.

**Example:**
```go
import "github.com/tnotstar/go-minolas/pkg/cli/argparse"

// Initialize parser
parser := argparse.NewArgumentParser("deployer", "Deploy applications")
	
// Flags
verbose := parser.Bool("--verbose", argparse.Short("v"), argparse.Help("Enable verbose mode"))
region := parser.String("--region", argparse.Default("us-east-1"))

// Positional arguments
appName := parser.String("app", argparse.Required())

// Subcommands
dbCmd := parser.NewCommand("db", "Database tools")
host := dbCmd.String("--host", argparse.Default("localhost"))

if err := parser.Parse(os.Args[1:]); err != nil {
    if err == argparse.ErrHelp {
        fmt.Print(parser.Usage())
        os.Exit(0)
    }
    fmt.Println(err)
    fmt.Print(parser.Usage())
    os.Exit(1)
}

if dbCmd.Invoked() {
    fmt.Println("Connecting to", *host)
}
```

### `pkg/db/sqlt`

Database connection utilities with URL-based configuration and a pluggable driver architecture built purely on `database/sql`.

**Features:**
- Open database connections using URL schemes
- Pluggable driver system via `Opener` interface
- Zero bundled third-party drivers: target executables import their own `database/sql` drivers and register matching `Opener` implementations
- Thread-safe driver registration

**Custom Drivers:**

Target applications register database drivers by implementing the `Opener` interface:

```go
type MyOpener struct{}

func (o *MyOpener) Id() string { return "mydriver" }
func (o *MyOpener) CanOpen(u *url.URL) bool { return u.Scheme == "mydriver" }
func (o *MyOpener) Open(u *url.URL) (*sql.DB, error) {
    // Custom opening logic using database/sql
}

func init() {
    sqlt.RegisterOpener(&MyOpener{})
}
```

Then open connections anywhere via:

```go
db, err := sqlt.Open("mydriver://localhost/mydb")
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

## Testing

Run all tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Run tests with race detection:
```bash
go test -race ./...
```

## License

See [LICENSE](LICENSE) file for details.
