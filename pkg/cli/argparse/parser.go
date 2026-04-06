package argparse

import (
	"fmt"
	"strconv"
	"strings"
)

// ArgumentParser is the main structure for parsing command-line arguments.
type ArgumentParser struct {
	Name        string
	Description string

	// parent parser if this is a subcommand
	parent *ArgumentParser

	// positional arguments
	posArgs []*argDef

	// optional flags (long and short)
	optArgs map[string]*argDef

	// registered subcommands
	subcommands map[string]*ArgumentParser

	// internal list of all defined options for preserving order in help
	allOptArgs []*argDef

	// track if this sub-parser was invoked
	invoked bool
}

// argType distinguishes between types of arguments supported natively
type argType string

const (
	typeString  argType = "string"
	typeInt     argType = "int"
	typeBool    argType = "bool"
	typeStrList argType = "stringList"
)

// argDef defines a single argument
type argDef struct {
	name       string
	short      string
	help       string
	required   bool
	defaultVal interface{}
	typ        argType

	destPtr interface{} // Pointer to destination variable (*string, *int, etc)

	// internal state
	seen bool
}

// Option defines a functional option for argument configuration.
type Option func(*argDef)

// Short sets the short flag name (e.g. "v" for -v).
func Short(s string) Option {
	return func(a *argDef) {
		a.short = s
	}
}

// Help sets the description of the argument.
func Help(h string) Option {
	return func(a *argDef) {
		a.help = h
	}
}

// Default sets a default value for the argument.
func Default(v interface{}) Option {
	return func(a *argDef) {
		a.defaultVal = v
	}
}

// Required ensures this argument must be provided.
func Required() Option {
	return func(a *argDef) {
		a.required = true
	}
}

// NewArgumentParser creates a new top-level argument parser.
func NewArgumentParser(name, description string) *ArgumentParser {
	return &ArgumentParser{
		Name:        name,
		Description: description,
		optArgs:     make(map[string]*argDef),
		subcommands: make(map[string]*ArgumentParser),
	}
}

func (p *ArgumentParser) isFlag(name string) bool {
	return strings.HasPrefix(name, "-")
}

func (p *ArgumentParser) addArg(name string, typ argType, dest interface{}, opts ...Option) {
	a := &argDef{
		name:    name,
		typ:     typ,
		destPtr: dest,
	}

	for _, opt := range opts {
		opt(a)
	}

	if p.isFlag(name) {
		// Strip leading dashes
		cleanName := strings.TrimLeft(name, "-")
		if cleanName == "" {
			panic("invalid flag name: " + name)
		}
		a.name = cleanName

		// If this is a re-registration, that's generally a bug by the user, but we will overwrite
		p.optArgs[cleanName] = a
		if a.short != "" {
			cleanShort := strings.TrimLeft(a.short, "-")
			p.optArgs[cleanShort] = a
			a.short = cleanShort
		}
		p.allOptArgs = append(p.allOptArgs, a)
	} else {
		p.posArgs = append(p.posArgs, a)
	}

	// Apply default values immediately
	if a.defaultVal != nil {
		p.applyVal(a, a.defaultVal)
	}
}

func (p *ArgumentParser) applyVal(a *argDef, val interface{}) {
	switch a.typ {
	case typeString:
		if v, ok := val.(string); ok {
			*a.destPtr.(*string) = v
		}
	case typeInt:
		if v, ok := val.(int); ok {
			*a.destPtr.(*int) = v
		}
	case typeBool:
		if v, ok := val.(bool); ok {
			*a.destPtr.(*bool) = v
		}
	case typeStrList:
		if v, ok := val.([]string); ok {
			*a.destPtr.(*[]string) = append([]string(nil), v...)
		}
	}
}

// String defines a string argument and returns a pointer to its parsed value.
func (p *ArgumentParser) String(name string, opts ...Option) *string {
	var v string
	p.addArg(name, typeString, &v, opts...)
	return &v
}

// Int defines an integer argument and returns a pointer to its parsed value.
func (p *ArgumentParser) Int(name string, opts ...Option) *int {
	var v int
	p.addArg(name, typeInt, &v, opts...)
	return &v
}

// Bool defines a boolean argument and returns a pointer to its parsed value.
func (p *ArgumentParser) Bool(name string, opts ...Option) *bool {
	var v bool
	p.addArg(name, typeBool, &v, opts...)
	return &v
}

// StringList defines a string list argument (which can be passed multiple times) and returns it.
func (p *ArgumentParser) StringList(name string, opts ...Option) *[]string {
	var v []string
	p.addArg(name, typeStrList, &v, opts...)
	return &v
}

// NewCommand creates a new subcommand argument parser.
func (p *ArgumentParser) NewCommand(name, description string) *ArgumentParser {
	cmd := NewArgumentParser(name, description)
	cmd.parent = p
	p.subcommands[name] = cmd
	return cmd
}

// Invoked returns true if this specific parser/subcommand was invoked.
func (p *ArgumentParser) Invoked() bool {
	return p.invoked
}

// parseEngine defines the internal parse logic
type parseEngine struct {
	args []string
	pos  int
}

func (e *parseEngine) next() (string, bool) {
	if e.pos >= len(e.args) {
		return "", false
	}
	v := e.args[e.pos]
	e.pos++
	return v, true
}

func (e *parseEngine) peek() (string, bool) {
	if e.pos >= len(e.args) {
		return "", false
	}
	return e.args[e.pos], true
}

// Parse processes the provided arguments and returns an error if any.
func (p *ArgumentParser) Parse(args []string) error {
	p.invoked = true

	e := &parseEngine{args: args, pos: 0}

	posIdx := 0

	for {
		arg, ok := e.next()
		if !ok {
			break
		}

		// check for subcommand if it's not a flag and we haven't seen positional elements
		// technically, in argparse, subcommands usually follow flags before pos params.
		// For simplicity, any non-flag token that matches a subcommand delegates.
		// To be stricter: it must be before any positional argument is consumed?
		// We'll allow it anywhere if it hasn't processed positionals, otherwise treat as positional.
		if !strings.HasPrefix(arg, "-") && len(p.subcommands) > 0 && posIdx == 0 {
			if subCmd, exists := p.subcommands[arg]; exists {
				// Delegate to subcommand
				return subCmd.Parse(e.args[e.pos:])
			}
		}

		if strings.HasPrefix(arg, "-") {
			// It is a flag
			if arg == "--" {
				// stop parameter parsing, rest are positional
				for {
					restArg, ok := e.next()
					if !ok {
						break
					}
					if err := p.handlePositional(restArg, &posIdx); err != nil {
						return err
					}
				}
				break
			}

			// Handle flag
			name := strings.TrimLeft(arg, "-")

			// Check for help flag
			if name == "h" || name == "help" {
				return ErrHelp
			}

			val := ""

			// check for --flag=value format
			if idx := strings.IndexByte(name, '='); idx != -1 {
				val = name[idx+1:]
				name = name[:idx]
			}

			def, exists := p.optArgs[name]
			if !exists {
				return fmt.Errorf("unknown argument: %s", arg)
			}

			def.seen = true

			if def.typ == typeBool {
				// boolean flags don't consume next arg unless it's in --flag=value form
				if val == "" {
					*def.destPtr.(*bool) = true
				} else {
					b, err := strconv.ParseBool(val)
					if err != nil {
						return fmt.Errorf("invalid boolean value for %s: %s", arg, val)
					}
					*def.destPtr.(*bool) = b
				}
				continue
			}

			// for non-bool, if val is empty, we must consume the next argument
			if val == "" {
				nextArg, hasNext := e.peek()
				if !hasNext || (strings.HasPrefix(nextArg, "-") && nextArg != "-") {
					return fmt.Errorf("argument %s requires a value", arg)
				}
				val, _ = e.next() // consume
			}

			if err := p.parseValue(def, val); err != nil {
				return fmt.Errorf("argument %s: %w", arg, err)
			}

		} else {
			// Positional
			if err := p.handlePositional(arg, &posIdx); err != nil {
				return err
			}
		}
	}

	// Validate required arguments
	return p.validateRequired()
}

func (p *ArgumentParser) handlePositional(val string, posIdx *int) error {
	if *posIdx >= len(p.posArgs) {
		return fmt.Errorf("unrecognized positional argument: %s", val)
	}
	def := p.posArgs[*posIdx]
	def.seen = true
	*posIdx++
	return p.parseValue(def, val)
}

func (p *ArgumentParser) parseValue(def *argDef, val string) error {
	switch def.typ {
	case typeString:
		*def.destPtr.(*string) = val
	case typeInt:
		parsed, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", val)
		}
		*def.destPtr.(*int) = parsed
	case typeStrList:
		*def.destPtr.(*[]string) = append(*def.destPtr.(*[]string), val)
	}
	return nil
}

func (p *ArgumentParser) validateRequired() error {
	for _, a := range p.posArgs {
		if a.required && !a.seen {
			return fmt.Errorf("missing required positional argument: %s", a.name)
		}
	}
	for _, a := range p.allOptArgs {
		if a.required && !a.seen {
			flagPrefix := "--"
			if len(a.name) == 1 {
				flagPrefix = "-"
			}
			return fmt.Errorf("missing required argument: %s%s", flagPrefix, a.name)
		}
	}
	return nil
}
