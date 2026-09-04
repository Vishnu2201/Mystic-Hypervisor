package cmd

import (
	"fmt"
	"io"
)

func runUninstall(w io.Writer, args []string) error {
	fmt.Fprintln(w, "mysticctl uninstall:")
	fmt.Fprintln(w, "  [NOT IMPLEMENTED] Uninstaller control plane functionality is scheduled for Milestone 2.")
	return nil
}
