// Package cli provides command-line interface utilities for building
// interactive CLI applications, including user input handling, confirmations,
// and option selection menus.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Confirm prompts the user with a yes/no question and returns true if the
// user confirms (answers 'y' or 'yes'), false otherwise.
// The prompt should not include the [y/N] suffix as it will be added automatically.
//
// Example:
//
//	if cli.Confirm("Do you want to continue?") {
//	    // User confirmed
//	}
func Confirm(prompt string) bool {
	return ConfirmWithDefault(prompt, false)
}

// ConfirmWithDefault prompts the user with a yes/no question and returns the
// specified default value if the user just presses Enter.
//
// Example:
//
//	if cli.ConfirmWithDefault("Delete file?", false) {
//	    // User confirmed or just pressed Enter (defaulting to false)
//	}
func ConfirmWithDefault(prompt string, defaultValue bool) bool {
	suffix := "[y/N]"
	if defaultValue {
		suffix = "[Y/n]"
	}

	fmt.Fprintf(os.Stdout, "%s %s ", prompt, suffix)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" {
		return defaultValue
	}

	return response == "y" || response == "yes"
}

// ReadInput prompts the user for input and returns the entered string.
// If a validator function is provided, the input is validated and the user
// is re-prompted if validation fails. Pass nil for validator to skip validation.
//
// Example:
//
//	name, err := cli.ReadInput("Enter your name: ", func(s string) error {
//	    if s == "" {
//	        return errors.New("name cannot be empty")
//	    }
//	    return nil
//	})
func ReadInput(prompt string, validator func(string) error) (string, error) {
	return ReadInputFromReader(os.Stdin, os.Stdout, prompt, validator)
}

// ReadInputFromReader is like ReadInput but allows specifying custom input and output.
// This is primarily useful for testing.
func ReadInputFromReader(in io.Reader, out io.Writer, prompt string, validator func(string) error) (string, error) {
	reader := bufio.NewReader(in)

	for {
		fmt.Fprint(out, prompt)

		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(input)

		if validator != nil {
			if err := validator(input); err != nil {
				fmt.Fprintf(out, "Invalid input: %v\n", err)
				continue
			}
		}

		return input, nil
	}
}

// SelectOption prompts the user to select an option from a list.
// Returns the zero-based index of the selected option and the option value.
// Returns an error if no options are provided or if input cannot be read.
//
// Example:
//
//	options := []string{"Option 1", "Option 2", "Option 3"}
//	idx, value, err := cli.SelectOption("Choose an option:", options)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("You selected: %s (index %d)\n", value, idx)
func SelectOption(prompt string, options []string) (int, string, error) {
	return SelectOptionFromReader(os.Stdin, os.Stdout, prompt, options)
}

// SelectOptionFromReader is like SelectOption but allows specifying custom input and output.
// This is primarily useful for testing.
func SelectOptionFromReader(in io.Reader, out io.Writer, prompt string, options []string) (int, string, error) {
	if len(options) == 0 {
		return -1, "", fmt.Errorf("no options provided")
	}

	fmt.Fprintln(out, prompt)
	for i, opt := range options {
		fmt.Fprintf(out, "  %d) %s\n", i+1, opt)
	}

	reader := bufio.NewReader(in)

	for {
		fmt.Fprint(out, "Select (1-", len(options), "): ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return -1, "", err
		}

		input = strings.TrimSpace(input)
		var choice int
		_, err = fmt.Sscanf(input, "%d", &choice)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Fprintf(out, "Invalid selection. Please enter a number between 1 and %d.\n", len(options))
			continue
		}

		idx := choice - 1
		return idx, options[idx], nil
	}
}
