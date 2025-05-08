package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// TestSayHello verifies that SayHello writes the expected greeting to stdout.
func TestSayHello(t *testing.T) {
	// Backup the real stdout
	oldStdout := os.Stdout
	// Create a pipe to capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	// Redirect stdout to the write end of the pipe
	os.Stdout = w

	// Call the function under test
	SayHello()

	// Close the writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read the captured output
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	// Define the expected output
	expected := "Hello, world!\n"
	// Compare and report
	if got := buf.String(); got != expected {
		t.Errorf("unexpected output: got %q, want %q", got, expected)
	}
}
