package plan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Message any

type Pipeline struct {
	Processors []Processor `yaml:"processors"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Pipeline. This
// is to allow the fancy named maps in a list.
func (p *Pipeline) UnmarshalYAML(value *yaml.Node) error {
	var aux struct {
		Processors []map[string]yaml.Node `yaml:"processors"`
	}
	if err := value.Decode(&aux); err != nil {
		return err
	}

	for _, procMap := range aux.Processors {
		if len(procMap) != 1 {
			return fmt.Errorf("each processor must have exactly one key, got %d", len(procMap))
		}
		for procType, procConfig := range procMap {
			var proc Processor
			switch procType {
			case "filter":
				var f Filter
				if err := procConfig.Decode(&f); err != nil {
					return fmt.Errorf("unmarshaling filter processor: %w", err)
				}
				proc = f
			case "transform":
				var t Transform
				if err := procConfig.Decode(&t); err != nil {
					return fmt.Errorf("unmarshaling transform processor: %w", err)
				}
				proc = t
			case "replace":
				var r Replace
				if err := procConfig.Decode(&r); err != nil {
					return fmt.Errorf("unmarshaling replace processor: %w", err)
				}
				proc = r
			case "map":
				var m Map
				if err := procConfig.Decode(&m); err != nil {
					return fmt.Errorf("unmarshaling map processor: %w", err)
				}
				proc = m
			default:
				return fmt.Errorf("unknown processor type: %s", procType)
			}
			p.Processors = append(p.Processors, proc)
		}
	}

	return nil
}

func deepCopy(data Message) (Message, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var msg Message
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// Process runs a message through all processors in the pipeline in order.
// If a processor returns nil, processing stops and nil is returned.
func (p Pipeline) Process(ctx context.Context, data Message) (Message, error) {
	data, err := deepCopy(data)
	if err != nil {
		return nil, fmt.Errorf("making deep copy of message: %w", err)
	}
	for _, processor := range p.Processors {
		var err error
		slog.Error("running processor", "type", fmt.Sprintf("%T", processor))
		data, err = processor.Process(ctx, data)
		switch {
		case err != nil:
			return nil, err
		case data == nil:
			return nil, nil
		}
	}
	return data, nil
}

type Processor interface {
	Process(ctx context.Context, msg Message) (Message, error)
}

// Transform modifies targeted fields in a message using Go templates.
type Transform struct {
	Fields map[string]string `yaml:"fields"`
}

// Process implements the Processor interface for Transform. The templates
// for each field will have the input message as its data context.
func (t Transform) Process(ctx context.Context, data Message) (Message, error) {
	reporter, ok := ctx.Value(reporterKey).(*Reporter)
	if !ok {
		return nil, fmt.Errorf("transform processor requires reporter in context")
	}

	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("transform processor expects input to be a map, got %T", data)
	}

	slog.Error("in transformer")
	for k, v := range t.Fields {
		slog.Error("transforming field", "key", k, "template", v)
		tmpl, err := template.New(k).Funcs(templateFuncs).Parse(v)
		if err != nil {
			slog.Error("unable to parse transform template", "key", k, "template", v, "err", err)
			return nil, fmt.Errorf("parsing template: %w", err)
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err != nil {
			return nil, fmt.Errorf("executing template: %w", err)
		}
		var v2 any
		if err := json.Unmarshal(out.Bytes(), &v2); err != nil {
			str := strings.TrimSpace(out.String())
			if str == "nil" {
				if m[k] != nil {
					reporter.Change(k, m[k], "<no value>")
				}
				delete(m, k)
				continue
			}
			slog.Debug("unable to unmarshal transform output, using raw string", "key", k, "output", str, "err", err)
			if m[k] != str {
				reporter.Change(k, m[k], str)
				m[k] = str
			}
		} else if m[k] != v2 {
			reporter.Change(k, m[k], v2)
			m[k] = v2
		}
	}

	return m, nil
}

// Replace completely replaces the input message with the output of a Go template.
type Replace struct {
	Template map[string]string `yaml:"template"`
}

// Process implements the Processor interface for Replace. The template provided
// will have the input message as its data context.
func (r Replace) Process(ctx context.Context, data Message) (Message, error) {
	out := make(map[string]any)

	for k, v := range r.Template {
		tmpl, err := template.New("template").Funcs(templateFuncs).Parse(v)
		if err != nil {
			return nil, fmt.Errorf("parsing template: %w", err)
		}

		var res bytes.Buffer
		if err := tmpl.Execute(&res, data); err != nil {
			return nil, fmt.Errorf("executing template: %w", err)
		}

		var v2 any
		if err := json.Unmarshal(res.Bytes(), &v2); err != nil {
			v2 = strings.TrimSpace(res.String())
		}
		out[k] = v2
	}
	return out, nil
}

var templateFuncs = template.FuncMap{
	"hasPrefix": strings.HasPrefix,
	"hasSuffix": strings.HasSuffix,
	"contains":  strings.Contains,
}

// Filter conditionally allows messages to continue through the pipeline based on
// prefix/suffix matching and/or a Go template query that evaluates to "true".
type Filter struct {
	Prefix map[string]string `yaml:"prefix"`
	Suffix map[string]string `yaml:"suffix"`
	Query  string            `yaml:"query"`
}

func (f Filter) suffixesMatch(ctx context.Context, data Message) bool {
	if f.Suffix == nil {
		return true
	}
	reporter, ok := ctx.Value(reporterKey).(*Reporter)
	if !ok {
		slog.Error("filter processor requires reporter in context - this is a bug in jsoninator")
		return false
	}

	for k, v := range f.Suffix {
		test, ok := dive(data, strings.Split(k, "."))
		if !ok {
			reporter.Skip(fmt.Sprintf("missing field %q for suffix check", k))
			return false
		}
		if s, ok := test.(string); !ok || !strings.HasSuffix(s, v) {
			reporter.Skip(fmt.Sprintf("field %q does not have suffix %q", k, v))
			return false
		}
	}

	return true
}

func (f Filter) prefixesMatch(ctx context.Context, data Message) bool {
	if f.Prefix == nil {
		return true
	}
	reporter, ok := ctx.Value(reporterKey).(*Reporter)
	if !ok {
		slog.Error("filter processor requires reporter in context - this is a bug in jsoninator")
		return false
	}

	for k, v := range f.Prefix {
		test, ok := dive(data, strings.Split(k, "."))
		if !ok {
			reporter.Skip(fmt.Sprintf("missing field %q for prefix check", k))
			return false
		}
		if s, ok := test.(string); !ok || !strings.HasPrefix(s, v) {
			reporter.Skip(fmt.Sprintf("field %q does not have prefix %q", k, v))
			return false
		}
	}

	return true
}

func (f Filter) queryMatches(ctx context.Context, data Message) bool {
	if f.Query == "" {
		return true
	}
	reporter, ok := ctx.Value(reporterKey).(*Reporter)
	if !ok {
		slog.Error("filter processor requires reporter in context - this is a bug in jsoninator")
		return false
	}

	tmpl, err := template.New("filter").Funcs(templateFuncs).Parse(f.Query)
	if err != nil {
		slog.Error("unable to parse query template '%s': %v", f.Query, err)
		return false
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		slog.Error("unable to execute query template '%s': %v", f.Query, err)
		return false
	}

	result := strings.TrimSpace(out.String())
	if result != "true" {
		reporter.Skip(fmt.Sprintf("query %q evaluated to %q", f.Query, result))
		return false
	}

	return true
}

// Process implements the Processor interface for Filter. Messages are evaluated
// against each of the filter's criteria, and if they pass all, the original message is
// returned. If any criteria are not met, nil is returned. Empty criteria are ignored.
func (f Filter) Process(ctx context.Context, data Message) (Message, error) {
	if f.prefixesMatch(ctx, data) &&
		f.suffixesMatch(ctx, data) &&
		f.queryMatches(ctx, data) {
		return data, nil
	}

	return nil, nil
}

// Map rewrites a message to be the value of a specified field, optionally
// providing a default if the field is not present.
type Map struct {
	Field   string `yaml:"field"`
	Default any    `yaml:"default,omitempty"`
}

func dive(data Message, selectors []string) (Message, bool) {
	if len(selectors) == 0 {
		return data, true
	}

	mm, ok := data.(map[string]any)
	if !ok {
		return nil, false
	}

	v, ok := mm[selectors[0]]
	if !ok {
		return nil, false
	}

	return dive(v, selectors[1:])
}

// Process implements the Processor interface for Map. The field specified
// will be extracted from the input message and returned. If the field is not
// present and a default is specified, the default will be returned. If the field
// is not present and no default is specified, nil is returned, so processing
// will stop.
func (m Map) Process(ctx context.Context, data Message) (Message, error) {
	selectors := strings.Split(m.Field, ".")

	msg, ok := dive(data, selectors)
	if !ok {
		return m.Default, nil
	}

	return msg, nil
}
