package plan

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

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
		return nil, err
	}
	for k, v := range i.HTTP.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (i Input) Read(ctx context.Context) ([]byte, error) {
	switch {
	case i.HTTP.URL != "":
		return i.readHTTP(ctx)
	case i.Raw != "":
		return []byte(i.Raw), nil
	}
	return nil, fmt.Errorf("no input source configured")
}
