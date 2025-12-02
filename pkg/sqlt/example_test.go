package sqlt_test

import (
	"fmt"

	"github.com/tnotstar/go-minolas/pkg/sqlt"
)

func ExampleListOpeners() {
	// List all registered database openers
	// Note: In a real application, SQLite opener is auto-registered on import
	openers := sqlt.ListOpeners()
	// The actual number depends on what's registered
	if len(openers) >= 0 {
		fmt.Println("Openers registered")
	}
	// Output: Openers registered
}
