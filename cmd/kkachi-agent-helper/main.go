package main

import (
	"os"

	"github.com/SeventeenthEarth/kkachi-agent-helper/internal/cli"
)

var (
	version   = "0.1.11"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	info := cli.NewBuildInfo("kkachi-agent-helper", version, commit, buildDate)

	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, info))
}
