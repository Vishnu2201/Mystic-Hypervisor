package cmd

import (
	"fmt"
	"io"
	"os"
)

var Version = "0.1.0-foundation"

// Execute runs the CLI subcommand parser.
func Execute(args []string) error {
	return ExecuteWriter(args, os.Stdout)
}

// ExecuteWriter allows testing output capture.
func ExecuteWriter(args []string, w io.Writer) error {
	if len(args) == 0 {
		printUsage(w)
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "version", "--version", "-v":
		return runVersion(w)
	case "status":
		return runStatus(w, subArgs)
	case "doctor":
		return runDoctor(w, subArgs)
	case "host":
		return runHost(w, subArgs)
	case "vm":
		return runVM(w, subArgs)
	case "container":
		return runContainer(w, subArgs)
	case "network":
		return runNetwork(w, subArgs)
	case "storage":
		return runStorage(w, subArgs)
	case "logs":
		return runLogs(w, subArgs)
	case "update":
		return runUpdate(w, subArgs)
	case "rollback":
		return runRollback(w, subArgs)
	case "uninstall":
		return runUninstall(w, subArgs)
	case "help", "--help", "-h":
		printUsage(w)
		return nil
	default:
		return fmt.Errorf("unknown command %q. Run 'mysticctl --help' for usage", subcommand)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "mysticctl — Mystic Hypervisor Command Line Tool")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  mysticctl <command> [arguments]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Available Commands:")
	fmt.Fprintln(w, "  status      Check Mystic daemon and provider status")
	fmt.Fprintln(w, "  doctor      Run system diagnostic checks")
	fmt.Fprintln(w, "  version     Display mysticctl version information")
	fmt.Fprintln(w, "  host        Manage host hypervisor nodes")
	fmt.Fprintln(w, "  vm          Manage virtual machines")
	fmt.Fprintln(w, "  container   Manage system containers")
	fmt.Fprintln(w, "  network     Manage virtual networks and bridges")
	fmt.Fprintln(w, "  storage     Manage storage pools and volumes")
	fmt.Fprintln(w, "  logs        Stream daemon audit and system logs")
	fmt.Fprintln(w, "  update      Update Mystic Hypervisor")
	fmt.Fprintln(w, "  rollback    Rollback Mystic installation")
	fmt.Fprintln(w, "  uninstall   Safely uninstall Mystic control plane")
}
