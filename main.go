package main

import (
	"fmt"
	"log"
	"os"

	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/core"
	"github.com/leafney/filedock/wire"
	"github.com/spf13/pflag"
)

var (
	Version   = "dev"
	GitBranch = "unknown"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	flags := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	configPath := config.DefaultPath()
	flags.StringVarP(&configPath, "config", "c", configPath, "config file")
	showVersion := flags.BoolP("version", "v", false, "show version")
	_ = flags.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("FileDock %s\n", Version)
		fmt.Printf("Git Branch: %s\n", GitBranch)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		return
	}

	app, err := wire.InitializeApp(configPath, core.BuildInfo{
		Version:   Version,
		Branch:    GitBranch,
		Commit:    GitCommit,
		BuildTime: BuildTime,
	})
	if err != nil {
		log.Printf("initialize filedock failed: %v", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		log.Printf("filedock stopped: %v", err)
		os.Exit(1)
	}
}
