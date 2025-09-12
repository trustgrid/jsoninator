package plan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"text/template"
)

// Output represents the output configuration for a run of jsoninator.
// All output methods are optional, and if multiple are specified,
// the processed message will be sent to all.
type Output struct {
	HTTP struct {
		URL         string            `yaml:"url"`
		Method      string            `yaml:"method"`
		Headers     map[string]string `yaml:"headers"`
		StatusCodes []int             `yaml:"status_codes"`
	} `yaml:"http"`

	Buffer *bytes.Buffer `yaml:"-"`
}

func (o Output) publishHTTP(ctx context.Context, original, processed Message) error {
	reader := bytes.NewBuffer(nil)
	if err := json.NewEncoder(reader).Encode(processed); err != nil {
		return err
	}

	tmpl, err := template.New("template").Funcs(templateFuncs).Parse(o.HTTP.URL)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, original); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, o.HTTP.Method, out.String(), reader)
	if err != nil {
		return err
	}
	for k, v := range o.HTTP.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if o.HTTP.StatusCodes != nil && !slices.Contains(o.HTTP.StatusCodes, resp.StatusCode) {
		slog.Error("unexpected status code", "status_code", resp.StatusCode, "expected", o.HTTP.StatusCodes)
		io.Copy(os.Stderr, resp.Body)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Publish sends the processed message to all configured output methods.
func (o Output) Publish(ctx context.Context, original, processed Message) error {
	if o.HTTP.URL != "" {
		if err := o.publishHTTP(ctx, original, processed); err != nil {
			return err
		}
	}
	if o.Buffer != nil {
		if err := json.NewEncoder(o.Buffer).Encode(processed); err != nil {
			return err
		}
	}

	return nil
}
