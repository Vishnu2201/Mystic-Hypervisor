package cmd

import (
	"fmt"
	"io"
)

func runRollback(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl rollback:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] System rollback functionality is scheduled for Milestone 2.")
	return nil
}
