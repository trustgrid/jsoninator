package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

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

func prompt() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Variable name (leave blank to finish): ")
		text, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("error reading input", "err", err)
			return
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		fmt.Print("Value: ")
		value, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("error reading input", "err", err)
			return
		}
		os.Setenv(text, strings.TrimSpace(value))
	}
}

func main() {
	setLogLevel()

	dryrun := flag.Bool("dryrun", true, "When set (the default), this will not write to any outputs")
	planFile := flag.String("plan", "", "Path to the plan YAML file")
	promptFlag := flag.Bool("prompt", false, "If set, will ask you for variables")
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

	if *promptFlag {
		prompt()
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
