package argparse

import (
	"strings"
	"testing"
)

func TestBasicParsing(t *testing.T) {
	p := NewArgumentParser("test", "A test desc")

	name := p.String("name", Required())
	count := p.Int("--count", Short("c"), Default(1))
	verbose := p.Bool("--verbose", Short("v"))
	flags := p.StringList("--flag", Short("f"))

	err := p.Parse([]string{"--count", "5", "--verbose", "-f", "a", "-f", "b", "myname"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *name != "myname" {
		t.Errorf("expected name 'myname', got %s", *name)
	}
	if *count != 5 {
		t.Errorf("expected count 5, got %d", *count)
	}
	if !*verbose {
		t.Errorf("expected verbose true, got false")
	}
	if len(*flags) != 2 || (*flags)[0] != "a" || (*flags)[1] != "b" {
		t.Errorf("expected flags [a, b], got %v", *flags)
	}
}

func TestSubcommands(t *testing.T) {
	p := NewArgumentParser("test", "A test parser")
	dbCmd := p.NewCommand("db", "db commands")
	host := dbCmd.String("--host", Default("localhost"))

	dbCmdPos := dbCmd.String("action", Required())

	err := p.Parse([]string{"db", "--host", "remote", "migrate"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dbCmd.Invoked() {
		t.Errorf("expected db command to be invoked")
	}

	if *host != "remote" {
		t.Errorf("expected host 'remote', got %s", *host)
	}
	if *dbCmdPos != "migrate" {
		t.Errorf("expected db action 'migrate', got %s", *dbCmdPos)
	}
}

func TestRequiredArgument(t *testing.T) {
	p := NewArgumentParser("test", "test")
	p.String("req", Required())

	err := p.Parse([]string{})
	if err == nil {
		t.Fatal("expected error for missing required argument")
	}
	if !strings.Contains(err.Error(), "missing required") {
		t.Errorf("expected missing required error, got: %v", err)
	}
}

func TestHelpFlag(t *testing.T) {
	p := NewArgumentParser("test", "")
	err := p.Parse([]string{"--help"})
	if err != ErrHelp {
		t.Errorf("expected ErrHelp, got %v", err)
	}
}

func TestFormatValues(t *testing.T) {
	p := NewArgumentParser("test", "")
	equalFlag := p.String("--eq")

	err := p.Parse([]string{"--eq=value"})
	if err != nil {
		t.Fatal(err)
	}
	if *equalFlag != "value" {
		t.Errorf("expected value, got %s", *equalFlag)
	}
}

func TestBooleanFormat(t *testing.T) {
	p := NewArgumentParser("test", "")
	b1 := p.Bool("--b1")
	b2 := p.Bool("--b2")

	err := p.Parse([]string{"--b1", "--b2=false"})
	if err != nil {
		t.Fatal(err)
	}

	if !*b1 {
		t.Errorf("expected b1 true")
	}
	if *b2 {
		t.Errorf("expected b2 false")
	}
}

func TestUnknownFlag(t *testing.T) {
	p := NewArgumentParser("test", "")
	p.String("pos")

	err := p.Parse([]string{"--unknown", "val"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestSeen(t *testing.T) {
	p := NewArgumentParser("test", "")
	prio := p.String("-p", Default("B"))
	verbose := p.Bool("--verbose", Short("v"))
	target := p.String("target")

	if err := p.Parse([]string{"-p", "B", "main"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *prio != "B" || *verbose {
		t.Fatalf("unexpected values: prio=%q verbose=%v", *prio, *verbose)
	}
	if !p.Seen("p") || !p.Seen("-p") {
		t.Error("expected -p to be marked as seen even with default value")
	}
	if p.Seen("verbose") || p.Seen("v") {
		t.Error("expected --verbose not to be marked as seen")
	}
	if !p.Seen("target") || *target != "main" {
		t.Error("expected positional target to be marked as seen")
	}
	if p.Seen("nonexistent") || p.Seen("-") {
		t.Error("expected unknown/empty flag names not to be seen")
	}
}

func TestEmptyEqualValueAndDashValues(t *testing.T) {
	p := NewArgumentParser("test", "")
	title := p.String("--title", Default("fallback"))
	count := p.Int("--count")
	pos := p.String("pos")

	if err := p.Parse([]string{"--title=", "--count", "-5", "item"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *title != "" {
		t.Errorf("expected empty title, got %q", *title)
	}
	if *count != -5 {
		t.Errorf("expected count -5, got %d", *count)
	}
	if *pos != "item" {
		t.Errorf("expected pos 'item', got %q", *pos)
	}

	// Dash-prefixed string value
	p2 := NewArgumentParser("test", "")
	t2 := p2.String("-title")
	if err := p2.Parse([]string{"-title", "-draft"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *t2 != "-draft" {
		t.Errorf("expected '-draft', got %q", *t2)
	}

	// Empty bool= value must fail
	p3 := NewArgumentParser("test", "")
	p3.Bool("--flag")
	if err := p3.Parse([]string{"--flag="}); err == nil {
		t.Fatal("expected error for --flag= with empty boolean value")
	}
}

func TestSubcommandHelpAndDeterministicUsage(t *testing.T) {
	p := NewArgumentParser("za", "root")
	p.NewCommand("zeta", "last")
	p.NewCommand("alpha", "first")
	sub := p.NewCommand("mid", "middle")

	if err := p.Parse([]string{"mid", "--help"}); err != ErrHelp {
		t.Errorf("expected ErrHelp from subcommand, got %v", err)
	}
	if !sub.Invoked() {
		t.Error("expected subcommand to be marked invoked")
	}

	usage := p.Usage()
	idxAlpha := strings.Index(usage, "alpha")
	idxMid := strings.Index(usage, "mid")
	idxZeta := strings.Index(usage, "zeta")
	if idxAlpha == -1 || idxMid == -1 || idxZeta == -1 || !(idxAlpha < idxMid && idxMid < idxZeta) {
		t.Errorf("expected deterministic alphabetical subcommands in Usage, got:\n%s", usage)
	}
}
