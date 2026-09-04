package cmd

import (
	"fmt"
	"io"
)

func runContainer(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl container:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] Container management is not yet implemented in Milestone 1.")
	return nil
}
