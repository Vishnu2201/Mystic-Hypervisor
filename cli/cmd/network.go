package cmd

import (
	"fmt"
	"io"
)

func runNetwork(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl network:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] Network management is not yet implemented in Milestone 1.")
	return nil
}
