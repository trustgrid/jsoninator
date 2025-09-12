package plan

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Input represents the input configuration for a run of jsoninator.
// Only one input method should be specified.
type Input struct {
	HTTP struct {
		URL     string            `yaml:"url"`
		Headers map[string]string `yaml:"headers"`
	} `yaml:"http"`

	Raw string `yaml:"raw"`
}

func (i Input) readHTTP(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.HTTP.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("constructing read request: %w", err)
	}
	for k, v := range i.HTTP.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending read request: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Read fetches the input data according to the configured input method.
// It's expected to return either a JSON array or JSON object.
func (i Input) Read(ctx context.Context) ([]byte, error) {
	switch {
	case i.HTTP.URL != "":
		return i.readHTTP(ctx)
	case i.Raw != "":
		return []byte(i.Raw), nil
	}
	return nil, fmt.Errorf("no input source configured")
}
