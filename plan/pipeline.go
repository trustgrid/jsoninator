package plan

import (
	"bytes"
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

func (p Pipeline) Process(data Message) (Message, error) {
	for _, processor := range p.Processors {
		var err error
		data, err = processor.Process(data)
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
	Process(Message) (Message, error)
}

type Transform struct {
	Fields map[string]string `yaml:"fields"`
}

func (t Transform) Process(data Message) (Message, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("transform processor expects input to be a map, got %T", data)
	}

	for k, v := range t.Fields {
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
			if out.String() == "nil" {
				delete(m, k)
				continue
			}
			slog.Debug("unable to unmarshal transform output, using raw string", "key", k, "output", out.String(), "err", err)
			m[k] = out.String()
		} else {
			m[k] = v2
		}
	}

	return m, nil
}

type Replace struct {
	Template string `yaml:"template"`
}

func (r Replace) Process(data Message) (Message, error) {
	tmpl, err := template.New("template").Funcs(templateFuncs).Parse(r.Template)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}
	var msg Message
	return msg, json.Unmarshal(out.Bytes(), &msg)
}

var templateFuncs = template.FuncMap{
	"hasPrefix": strings.HasPrefix,
	"hasSuffix": strings.HasSuffix,
	"contains":  strings.Contains,
}

type Filter struct {
	Query string `yaml:"query"`
}

func (f Filter) Process(data Message) (Message, error) {
	tmpl, err := template.New("filter").Funcs(templateFuncs).Parse(f.Query)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}
	if strings.TrimSpace(out.String()) == "true" {
		return data, nil
	}
	return nil, nil
}

type Map struct {
	Field   string `yaml:"field"`
	Default any    `yaml:"default,omitempty"`
}

func (m Map) dive(data Message, selectors []string) (Message, bool) {
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

	return m.dive(v, selectors[1:])
}

func (m Map) Process(data Message) (Message, error) {
	selectors := strings.Split(m.Field, ".")

	msg, ok := m.dive(data, selectors)
	if !ok {
		return m.Default, nil
	}

	return msg, nil
}
