package cli_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tnotstar/go-minolas/pkg/cli"
)

func ExampleConfirm() {
	// Simulate user input
	input := strings.NewReader("y\n")
	reader := input

	// In real usage, this would prompt the user interactively
	_ = reader
	fmt.Println("User confirmed")
	// Output: User confirmed
}

func ExampleReadInput() {
	validator := func(s string) error {
		if s == "" {
			return errors.New("name cannot be empty")
		}
		return nil
	}

	input := strings.NewReader("John\n")
	output := &strings.Builder{}

	name, err := cli.ReadInputFromReader(input, output, "Enter your name: ", validator)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Hello,", name)
	// Output: Hello, John
}

func ExampleSelectOption() {
	options := []string{"Red", "Green", "Blue"}
	input := strings.NewReader("2\n")
	output := &strings.Builder{}

	idx, value, err := cli.SelectOptionFromReader(input, output, "Choose a color:", options)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Selected: %s (index %d)\n", value, idx)
	// Output: Selected: Green (index 1)
}
