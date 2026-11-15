// Command diff-coverage gates the coverage of changed lines.
package main

import (
	"os"

	"github.com/xdoubleu/diff-coverage/internal/diffcov"
)

func main() {
	os.Exit(diffcov.Run(os.Args[1:], os.Stdout, os.Stderr))
}
