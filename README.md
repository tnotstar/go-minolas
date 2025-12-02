# go-minolas

A collection of reusable Go utilities for common tasks in daily development work.

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
    return nil
})
```

### `pkg/sqlt`

Database connection utilities with URL-based configuration and a pluggable driver architecture.

**Features:**
- Open database connections using URL schemes
- Pluggable driver system via `Opener` interface
- Built-in SQLite support (via modernc.org/sqlite)
- Thread-safe driver registration

**Example:**
```go
import "github.com/tnotstar/go-minolas/pkg/sqlt"

// Open an in-memory SQLite database
db, err := sqlt.Open("sqlite::memory:")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Open a file-based SQLite database with options
db, err = sqlt.Open("sqlite:mydb.db?mode=rwc&cache=shared")
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

**Supported schemes:**
- `sqlite://` or `sqlite3://` - SQLite databases

**Custom Drivers:**

You can register custom database drivers by implementing the `Opener` interface:

```go
type MyOpener struct{}

func (o *MyOpener) Id() string { return "mydriver" }
func (o *MyOpener) CanOpen(u *url.URL) bool { return u.Scheme == "mydriver" }
func (o *MyOpener) Open(u *url.URL) (*sql.DB, error) {
    // Custom opening logic
}

func init() {
    sqlt.RegisterOpener(&MyOpener{})
}
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
