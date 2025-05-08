package cli

import "os"

func SayHello() {
	os.Stdout.Write([]byte("Hello, world!\n"))
}
