package argparse

import (
	"errors"
	"fmt"
	"strings"
)

// ErrHelp is returned when the --help or -h flag is encountered.
var ErrHelp = errors.New("help requested")

// Usage generates a formatted help string for the parser.
func (p *ArgumentParser) Usage() string {
	var sb strings.Builder

	// Build Usage line
	sb.WriteString("Usage: ")

	// Collect parser chain
	chain := p.getChain()
	for i, c := range chain {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(c.Name)
	}

	if len(p.allOptArgs) > 0 {
		sb.WriteString(" [options]")
	}

	if len(p.subcommands) > 0 {
		sb.WriteString(" <command> [args...]")
	}

	for _, a := range p.posArgs {
		if a.required {
			sb.WriteString(fmt.Sprintf(" <%s>", a.name))
		} else {
			sb.WriteString(fmt.Sprintf(" [%s]", a.name))
		}
	}
	sb.WriteString("\n\n")

	if p.Description != "" {
		sb.WriteString(p.Description)
		sb.WriteString("\n\n")
	}

	// Positional arguments
	if len(p.posArgs) > 0 {
		sb.WriteString("Positional Arguments:\n")
		for _, a := range p.posArgs {
			p.writeArgHelp(&sb, a, false)
		}
		sb.WriteString("\n")
	}

	// Options
	if len(p.allOptArgs) > 0 {
		sb.WriteString("Options:\n")
		// Automatically include the help flag definition
		sb.WriteString("  -h, --help           Show this help message and exit\n")

		for _, a := range p.allOptArgs {
			p.writeArgHelp(&sb, a, true)
		}
		sb.WriteString("\n")
	} else {
		// Still mention help
		sb.WriteString("Options:\n")
		sb.WriteString("  -h, --help           Show this help message and exit\n\n")
	}

	// Subcommands
	if len(p.subcommands) > 0 {
		sb.WriteString("Commands:\n")
		for name, cmd := range p.subcommands {
			sb.WriteString(fmt.Sprintf("  %-20s %s\n", name, cmd.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (p *ArgumentParser) getChain() []*ArgumentParser {
	var chain []*ArgumentParser
	curr := p
	for curr != nil {
		chain = append([]*ArgumentParser{curr}, chain...)
		curr = curr.parent
	}
	return chain
}

func (p *ArgumentParser) writeArgHelp(sb *strings.Builder, a *argDef, isOpt bool) {
	var flags string
	if isOpt {
		var parts []string
		if a.short != "" {
			parts = append(parts, "-"+a.short)
		}
		parts = append(parts, "--"+a.name)
		flags = strings.Join(parts, ", ")
	} else {
		flags = a.name
	}

	// Ensure spacing alignment
	if len(flags) > 20 {
		sb.WriteString(fmt.Sprintf("  %s\n  %-20s %s", flags, "", a.help))
	} else {
		sb.WriteString(fmt.Sprintf("  %-20s %s", flags, a.help))
	}

	var extras []string
	if a.required {
		extras = append(extras, "required")
	}
	if a.defaultVal != nil {
		extras = append(extras, fmt.Sprintf("default: %v", a.defaultVal))
	}

	if len(extras) > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(extras, ", ")))
	}
	sb.WriteString("\n")
}
