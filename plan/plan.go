package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// Plan represents the entire configuration for a run of jsoninator.
type Plan struct {
	Input        Input    `yaml:"input"`
	Pipeline     Pipeline `yaml:"pipeline"`
	Output       Output   `yaml:"output"`
	DryRun       bool     `yaml:"-"`
	skipReporter bool     `yaml:"-"`
}

// Parse parses a Plan from YAML data. Environment variables in the YAML
// are expanded before parsing.
func Parse(data []byte) (Plan, error) {
	var plan Plan
	expanded := os.ExpandEnv(string(data))
	return plan, yaml.Unmarshal([]byte(expanded), &plan)
}

func id(msg Message) string {
	m, ok := msg.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", msg)
	}

	keys := []string{"fqdn", "uid", "name", "id"}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}

	return fmt.Sprintf("%v", msg)
}

func (p Plan) processMsg(ctx context.Context, msg Message) error {
	ctx, cancel := WithReporter(ctx, id(msg))
	defer cancel()
	fmt.Println("Processing", id(msg))
	processed, err := p.Pipeline.Process(ctx, msg)
	switch {
	case err != nil:
		return err
	case processed == nil:
		return nil
	case p.DryRun:
		return nil
	}

	return p.Output.Publish(context.Background(), msg, processed)
}

// Run executes the plan: it reads input, processes messages through the pipeline,
// and publishes the output.
func (p Plan) Run(ctx context.Context) error {
	if !p.skipReporter {
		done := make(chan struct{})
		go func() {
			report(ctx)
			close(done)
		}()

		defer func() {
			closeReporters()
			<-done
		}()
	}

	inputData, err := p.Input.Read(ctx)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	var original Message
	var message Message
	if err := json.Unmarshal(inputData, &message); err != nil {
		slog.Error("unexpected input format", "input", string(inputData), "err", err)
		return fmt.Errorf("parsing input: %w", err)
	}
	if err := json.Unmarshal(inputData, &original); err != nil {
		slog.Error("unexpected input format", "input", string(inputData), "err", err)
		return fmt.Errorf("parsing input: %w", err)
	}

	switch msg := message.(type) {
	case []any:
		for _, item := range msg {
			if err := p.processMsg(ctx, item); err != nil {
				return fmt.Errorf("processing message: %w", err)
			}
		}
		return nil
	default:
		err := p.processMsg(ctx, msg)
		return err
	}
}
