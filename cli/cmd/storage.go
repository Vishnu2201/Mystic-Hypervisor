package cmd

import (
	"fmt"
	"io"
)

func runStorage(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl storage:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] Storage management is not yet implemented in Milestone 1.")
	return nil
}
