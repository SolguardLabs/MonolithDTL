package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "--help", "-h", "help":
		return usage()
	case "--list", "list":
		for _, name := range ListScenarios() {
			fmt.Println(name)
		}
		return nil
	case "scenario":
		if len(args) != 2 {
			return fmt.Errorf("usage: monolithdtl scenario <name>")
		}
		return PrintScenario(args[1])
	default:
		if strings.HasPrefix(args[0], "scenario=") {
			return PrintScenario(strings.TrimPrefix(args[0], "scenario="))
		}
		return fmt.Errorf("unknown command %s", args[0])
	}
}

func usage() error {
	fmt.Println("MonolithDTL")
	fmt.Println("usage:")
	fmt.Println("  monolithdtl --list")
	fmt.Println("  monolithdtl scenario <name>")
	return nil
}

