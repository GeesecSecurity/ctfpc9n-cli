package main

import (
	"os"

	"ctfpc9n-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdin, os.Stdout))
}
