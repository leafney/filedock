package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/leafney/filedock/config"
	"github.com/leafney/filedock/core"
	"github.com/leafney/filedock/wire"
)

var (
	Version   = "dev"
	GitBranch = "unknown"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	configPath := config.DefaultPath()
	flags.StringVar(&configPath, "config", configPath, "config file")
	showVersion := flags.Bool("version", false, "show version")
	_ = flags.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("filedock %s\n", Version)
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
