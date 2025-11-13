package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
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

	fmt.Print("Enter API key ID (TRUSTGRID_API_KEY_ID): ")
	key, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("error reading input", "err", err)
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		slog.Warn("API key ID is blank")
	} else {
		if err := os.Setenv("TRUSTGRID_API_KEY_ID", key); err != nil {
			slog.Error("error setting TRUSTGRID_API_KEY_ID envvar", "err", err)
		}
	}

	fmt.Print("Enter API key secret (TRUSTGRID_API_KEY_SECRET): ")
	secretBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		slog.Error("Unable to read API key secret", "err", err)
		return
	}

	secret := strings.TrimSpace(string(secretBytes))
	if secret == "" {
		slog.Warn("API key secret is blank")
	} else {
		if err := os.Setenv("TRUSTGRID_API_KEY_SECRET", secret); err != nil {
			slog.Error("error setting TRUSTGRID_API_KEY_SECRET envvar", "err", err)
		}
	}
}

func main() {
	setLogLevel()

	dryrun := flag.Bool("dryrun", true, "When set (the default), this will not write to any outputs")
	planFile := flag.String("plan", "", "Path to the plan YAML file")
	promptFlag := flag.Bool("prompt", false, "If set, you'll be prompted for an API key and secret")
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
