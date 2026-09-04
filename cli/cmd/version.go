package cmd

import (
	"fmt"
	"io"
)

func runVersion(w io.Writer) error {
	fmt.Fprintf(w, "mysticctl version %s (Milestone 1 — Engineering Foundation)\n", Version)
	return nil
}
