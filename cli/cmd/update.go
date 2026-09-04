package cmd

import (
	"fmt"
	"io"
)

func runUpdate(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl update:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] System update functionality is scheduled for Milestone 2.")
	return nil
}
