package cmd

import (
	"fmt"
	"io"
)

func runVM(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl vm:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] Virtual machine management is not yet implemented in Milestone 1.")
	return nil
}
