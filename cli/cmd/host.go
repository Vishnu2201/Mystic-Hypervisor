package cmd

import (
	"fmt"
	"io"
)

func runHost(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl host:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] Host node management functionality is not yet implemented in Milestone 1.")
	return nil
}
