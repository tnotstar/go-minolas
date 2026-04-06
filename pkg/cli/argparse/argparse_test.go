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
