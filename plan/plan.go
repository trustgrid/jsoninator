package plan

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Plan struct {
	Input    Input    `yaml:"input"`
	Pipeline Pipeline `yaml:"pipeline"`
	Output   io.Writer
}

func Parse(data []byte) (Plan, error) {
	var plan Plan
	expanded := os.ExpandEnv(string(data))
	return plan, yaml.Unmarshal([]byte(expanded), &plan) //nolint:musttag // no
}

func (p Plan) Run(ctx context.Context) error {
	inputData, err := p.Input.Read(ctx)
	if err != nil {
		return err
	}

	var original Message
	var message Message
	if err := json.Unmarshal(inputData, &message); err != nil {
		return err
	}
	if err := json.Unmarshal(inputData, &original); err != nil {
		return err
	}

	var output Message

	switch msg := message.(type) {
	case []any:
		var results []any
		for _, item := range msg {
			res, err := p.Pipeline.Process(item)
			switch {
			case err != nil:
				return err
			case res != nil:
				results = append(results, res)
			}
		}
		output = results
	default:
		res, err := p.Pipeline.Process(message)
		if err != nil {
			return err
		}
		output = res
	}

	enc := json.NewEncoder(p.Output)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
