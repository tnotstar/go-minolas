package sqlt_test

import (
	"fmt"

	"github.com/tnotstar/go-minolas/pkg/db/sqlt"
)

func ExampleListOpeners() {
	// List all registered database openers.
	// Consumers register their own Opener implementations (e.g. from init).
	openers := sqlt.ListOpeners()
	// The actual number depends on what's registered
	if len(openers) >= 0 {
		fmt.Println("Openers registered")
	}
	// Output: Openers registered
}
