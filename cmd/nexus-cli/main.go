// Command nexus-cli governs Nexus Repository 3.76 guest/anonymous access.
//
// See doc/nexus-cli第一版本PRD.md for the full product spec.
package main

import (
	"fmt"
	"os"

	"github.com/moge/nexus-cli/internal/cli"
)

func main() {
	root := buildRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func buildRoot() *cli.Root {
	return cli.NewRoot()
}
