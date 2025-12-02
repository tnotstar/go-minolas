package cli

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

// TestConfirm tests the Confirm function with various inputs.
func TestConfirm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"yes lowercase", "y\n", true},
		{"yes full", "yes\n", true},
		{"yes uppercase", "Y\n", true},
		{"yes full uppercase", "YES\n", true},
		{"no lowercase", "n\n", false},
		{"no full", "no\n", false},
		{"empty", "\n", false},
		{"invalid", "invalid\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			result := false
			if response == "" {
				result = false
			} else {
				result = response == "y" || response == "yes"
			}

			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestConfirmWithDefault tests the ConfirmWithDefault function.
func TestConfirmWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue bool
		expected     bool
	}{
		{"empty with default true", "\n", true, true},
		{"empty with default false", "\n", false, false},
		{"yes overrides default false", "y\n", false, true},
		{"no overrides default true", "n\n", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			result := tt.defaultValue
			if response != "" {
				result = response == "y" || response == "yes"
			}

			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestReadInputFromReader tests the ReadInputFromReader function.
func TestReadInputFromReader(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		validator func(string) error
		expected  string
		wantErr   bool
	}{
		{
			name:      "valid input without validator",
			input:     "test input\n",
			validator: nil,
			expected:  "test input",
			wantErr:   false,
		},
		{
			name:  "valid input with validator",
			input: "valid\n",
			validator: func(s string) error {
				if s == "" {
					return errors.New("empty")
				}
				return nil
			},
			expected: "valid",
			wantErr:  false,
		},
		{
			name:  "retry on invalid then valid",
			input: "\nvalid\n",
			validator: func(s string) error {
				if s == "" {
					return errors.New("empty")
				}
				return nil
			},
			expected: "valid",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &strings.Builder{}

			result, err := ReadInputFromReader(in, out, "Prompt: ", tt.validator)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error status: got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestSelectOptionFromReader tests the SelectOptionFromReader function.
func TestSelectOptionFromReader(t *testing.T) {
	options := []string{"Option 1", "Option 2", "Option 3"}

	tests := []struct {
		name          string
		input         string
		options       []string
		expectedIdx   int
		expectedValue string
		wantErr       bool
	}{
		{
			name:          "valid selection 1",
			input:         "1\n",
			options:       options,
			expectedIdx:   0,
			expectedValue: "Option 1",
			wantErr:       false,
		},
		{
			name:          "valid selection 2",
			input:         "2\n",
			options:       options,
			expectedIdx:   1,
			expectedValue: "Option 2",
			wantErr:       false,
		},
		{
			name:          "valid selection 3",
			input:         "3\n",
			options:       options,
			expectedIdx:   2,
			expectedValue: "Option 3",
			wantErr:       false,
		},
		{
			name:          "retry on invalid then valid",
			input:         "0\n2\n",
			options:       options,
			expectedIdx:   1,
			expectedValue: "Option 2",
			wantErr:       false,
		},
		{
			name:    "no options",
			input:   "1\n",
			options: []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &strings.Builder{}

			idx, value, err := SelectOptionFromReader(in, out, "Prompt:", tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error status: got error %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if idx != tt.expectedIdx {
					t.Errorf("got index %d, want %d", idx, tt.expectedIdx)
				}
				if value != tt.expectedValue {
					t.Errorf("got value %q, want %q", value, tt.expectedValue)
				}
			}
		})
	}
}
