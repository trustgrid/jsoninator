# jsoninator — Agent & Developer Guide

## What is jsoninator?

jsoninator is a CLI tool that runs a JSON data pipeline defined in a YAML plan file. It fetches JSON from an input source, passes each item through a sequence of processors, and sends results to an output destination.

Primary use case: bulk-updating Trustgrid node configurations via the Trustgrid portal API.

## Build & Run

```bash
# Build
make build          # produces bin/jsoninator
go build -o bin/jsoninator main.go

# Run (dry-run by default — no writes)
jsoninator -plan=examples/15-complete-workflow.yaml

# Run with real writes
jsoninator -plan=my-plan.yaml -dryrun=false

# Prompt for API credentials interactively
jsoninator -plan=my-plan.yaml -prompt
```

## Tests

```bash
make test
# or
go test -race -v -timeout 20s ./...
```

## Lint

```bash
make lint   # requires golangci-lint
```

## Environment Variables

Environment variables are expanded in plan YAML files (both values and URL templates):

```bash
export TRUSTGRID_API_KEY_ID="your_key_id"
export TRUSTGRID_API_KEY_SECRET="your_secret"
```

Set `LOG_LEVEL` (e.g. `debug`, `info`, `warn`, `error`) to control log verbosity.

## Plan File Structure

A plan YAML file has three top-level sections:

```yaml
input:    # where to fetch JSON
pipeline: # list of processors applied to each item
output:   # where to send each processed item
```

### Input

| Type   | Description                             |
|--------|-----------------------------------------|
| `http` | HTTP GET — response must be JSON object or array |
| `raw`  | Inline plaintext JSON                   |

### Pipeline Processors (applied in order)

| Processor   | Purpose                                               |
|-------------|-------------------------------------------------------|
| `filter`    | Drop items that don't match prefix/suffix/query rules |
| `map`       | Narrow to a nested field; subsequent processors see the inner object |
| `transform` | Patch specific fields using Go templates; `nil` removes a field |
| `replace`   | Emit a wholly new object built from templates         |

Go templates use the current item as `.`. All numbers are floats in templates.

### Output

| Type   | Description                          |
|--------|--------------------------------------|
| `http` | HTTP PUT/POST/PATCH to a URL         |

The output URL supports Go templates with the **original** (pre-map) message as context.

## Reports

jsoninator writes three CSV files to a datestamped directory under `reports/`:

- `changes.csv` — field-level diffs for transformed items
- `filtered.csv` — items excluded by a filter processor
- `noops.csv` — items that passed the pipeline but had no changes

## Key Source Files

| File                  | Purpose                                      |
|-----------------------|----------------------------------------------|
| `main.go`             | CLI entry point; flag parsing, prompt helper |
| `plan/plan.go`        | Plan struct, `Parse()`, `Run()`              |
| `plan/pipeline.go`    | Processor chain execution                    |
| `plan/input.go`       | HTTP and raw input sources                   |
| `plan/output.go`      | HTTP output                                  |
| `plan/reporter.go`    | CSV report writing                           |
| `examples/`           | Runnable YAML plan examples                  |

## Code Conventions

- All pipeline logic lives in the `plan` package.
- Processors implement a common interface; add new ones in `plan/pipeline.go`.
- Templates use `text/template`; whitespace is trimmed from evaluated values.
- `nil` as a template result removes the field from the object.
