package main

import (
	"fmt"
	"os"

	"github.com/mystic-hypervisor/mystic/cli/cmd"
)

func main() {
	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
