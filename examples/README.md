# Jsoninator Examples

This directory contains example YAML plans demonstrating various features and use cases of jsoninator.

## Basic Examples

### 01-basic-http-input.yaml
Demonstrates fetching JSON data from an HTTP endpoint with headers and environment variable substitution.

### 02-raw-input.yaml
Shows how to use inline JSON data as input, useful for testing pipelines.

## Filtering Examples

### 03-filter-prefix.yaml
Filter data based on field prefixes (e.g., names starting with "gw-").

### 04-filter-suffix.yaml
Filter data based on field suffixes (e.g., names ending with "-prod").

### 05-filter-query.yaml
Use Go template expressions for complex filtering logic (e.g., age > 65).

### 06-filter-combined.yaml
Combine multiple filter conditions (prefix + query).

### 07-filter-isblank-isset.yaml
Use `isSet` and `isBlank` helper functions to handle missing or empty fields.

## Data Transformation Examples

### 08-map-basic.yaml
Extract specific fields or nested objects from input data.

### 09-map-with-default.yaml
Use default values when mapped fields are missing.

### 10-transform-basic.yaml
Modify, add, or compute new fields using templates.

### 11-transform-defaults.yaml
Advanced transform usage: defaults, conditional values, and field deletion.

### 12-replace-basic.yaml
Create entirely new objects with only specified fields.

## Pipeline Examples

### 13-multi-stage-pipeline.yaml
Chain multiple processors together for complex transformations.

### 14-http-output.yaml
Send processed data to HTTP endpoints.

### 15-complete-workflow.yaml
Full ETL workflow: fetch from API, transform data, send to API.

### 16-lifecycle-update.yaml
Shows how to set the lifecycleStatus on devices based on their prod_status tag, and uses the `hasTag` helper.

### 17-lifecycle-from-tag.yaml
Migrates `prod_status` / `ProdStatus` tags to the lifecycleState attribute, demonstrating how to match multiple tag names and spelling variants (e.g. "pre-production" vs "preproduction") with chained `or` / `hasTag` calls.

### 18-remove-lifecycle-tags.yaml
Removes the legacy `prod_status` or `ProdStatus` tag from nodes whose lifecycleState is already set. Run once per tag name via the `LIFECYCLE_TAG_NAME` environment variable. Execute this plan **after** updating alarm filters to use lifecycleState.

## Running Examples

All examples can be run with:

```bash
jsoninator -plan=examples/[example-file].yaml
```

By default, jsoninator runs in dry-run mode (no outputs are written). To execute with real outputs:

```bash
jsoninator -plan=examples/[example-file].yaml -dryrun=false
```

## Environment Variables

Many examples use environment variables for sensitive data. Set them before running:

**Linux/macOS:**
```bash
export TRUSTGRID_API_KEY_ID="your_key_id"
export TRUSTGRID_API_KEY_SECRET="your_secret"
```

**Windows PowerShell:**
```powershell
$env:TRUSTGRID_API_KEY_ID="your_key_id"
$env:TRUSTGRID_API_KEY_SECRET="your_secret"
```

## Template Syntax

Examples use Go template syntax. Key points:

- Access fields with `.fieldName`
- Access nested fields with `.parent.child`
- All numbers are floats (use `.0` suffix, e.g., `65.0`)
- Return `"nil"` from templates to delete fields
- Whitespace in templates is trimmed automatically

### Common Template Functions

- `eq`: equals comparison
- `gt`: greater than
- `lt`: less than
- `hasPrefix`: check string prefix
- `hasSuffix`: check string suffix
- `isSet`: check if field exists
- `isBlank`: check if string is empty

## Outputs and Reporting

When running examples, jsoninator creates:

- **stdout**: Progress messages and status
- **stderr**: Debug and error logs
- **reports/**: Directory with CSV files:
  - `changes.csv`: Records of field transformations
  - `filtered.csv`: Items that were filtered out
  - `noops.csv`: Items that passed through unchanged

## Learn More

See the main [README.md](../README.md) for complete documentation.
