// Command compatlab is the CompatLab command-line interface.
package main

import (
	"os"

	"github.com/thanhmaitran/compatlab/apps/cli/commands"
)

func main() {
	os.Exit(commands.Run(os.Args[1:], os.Stdout, os.Stderr))
}
