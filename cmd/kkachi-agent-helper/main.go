package main

import (
	"os"

	"github.com/SeventeenthEarth/kkachi-agent-helper/internal/cli"
)

var (
	version   = "0.0.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	info := cli.BuildInfo{
		Name:      "kkachi-agent-helper",
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}

	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, info))
}
