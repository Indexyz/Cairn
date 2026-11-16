// Command cairn-dispatcher runs the Cairn dispatcher.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
	"github.com/Indexyz/Cairn/dispatcher/internal/scheduler"
)

func main() {
	configPath := flag.String("config", "", "dispatcher config path")
	once := flag.Bool("once", false, "run one scheduling iteration and exit")
	startupHealthcheckOnly := flag.Bool("startup-healthcheck-only", false, "run startup worker healthchecks and exit")
	logLevel := flag.String("log-level", "INFO", "log level")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "error: --config is required")
		flag.Usage()
		os.Exit(2)
	}

	logging.Configure(logging.ParseLevel(*logLevel), *startupHealthcheckOnly)

	loop, err := scheduler.New(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *startupHealthcheckOnly {
		err = loop.RunStartupHealthchecksOnly()
	} else {
		err = loop.Run(*once)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
