package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"trustgrid.io/jsoninator/plan"
)

func setLogLevel() {
	if level, ok := os.LookupEnv("LOG_LEVEL"); ok {
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(level)); err != nil {
			slog.Error("invalid LOG_LEVEL", "err", err)
			return
		}
		slog.SetLogLoggerLevel(lvl)
	}
}

func main() {
	setLogLevel()

	dryrun := flag.Bool("dryrun", true, "When set (the default), this will not write to any outputs")
	planFile := flag.String("plan", "", "Path to the plan YAML file")
	flag.Parse()

	if *planFile == "" {
		fmt.Println("You must provide a plan file with -plan")
		return
	}
	f, err := os.ReadFile(*planFile)
	if err != nil {
		slog.Error("unable to read plan file", "err", err)
	}

	if *dryrun {
		fmt.Println("DRY RUN ENABLED: No outputs will be written to")
	}

	program, err := plan.Parse(f)
	if err != nil {
		slog.Error("unable to parse plan file", "err", err)
		return
	}

	program.DryRun = *dryrun

	if err := program.Run(context.Background()); err != nil {
		slog.Error("error running plan", "err", err)
	}
}
