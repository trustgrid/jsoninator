
# JSONINATOR

jsoninator is a tool that reads JSON from an input, runs the JSON through different pipeline processors, and then publishes the resulting output somewhere.

Currently input sources are limited to HTTP and plaintext. Output destinations are limited to HTTP.

Input, processors, and outputs are managed in a plan YAML file. 

Below is a plan YAML that would update all appliances who have a name that start with "gw-" to have UDP enabled, ensuring the UDP port is set and that invalid values for `cert` and `clientMaxEgressMbps` are omitted:

```yaml
input:
  http:
    url: https://portal.trustgrid.io/api/node?projection[0]=uid&projection[1]=name&projection[2]=tags&projection[3][0]=config&projection[3][1]=gateway&projection[4]=fqdn
    headers:
      Authorization: "trustgrid-token ${TRUSTGRID_API_KEY_ID}:${TRUSTGRID_API_KEY_SECRET}"
      Accept: application/json

pipeline:
  processors:
    - filter:
        prefix:
          name: gw-
        query: |
          {{if (eq .type "Node")}}true{{else}}false{{end}}
    - map:
        field: config.gateway
    - transform:
        fields:
          udpEnabled: true
          udpPort: |
            {{if .udpPort}}{{.udpPort}}{{else if .port}}{{.port}}{{else}}8995{{end}}
          maxClientWriteMbps: |
            {{if or (eq .maxClientWriteMbps 0.0) (not .maxClientWriteMbps)}}nil{{else}}{{.maxClientWriteMbps}}{{end}}
          cert: |
            {{if .cert}}{{.cert}}{{else}}nil{{end}}

output:
  http:
    url: https://portal.trustgrid.io/api/node/{{.uid}}/config/gateway
    method: PUT
    status_codes: [200]
    headers:
      Authorization: "trustgrid-token ${TRUSTGRID_API_KEY_ID}:${TRUSTGRID_API_KEY_SECRET}"
      Content-Type: application/json
```

**Note that environment variables are expanded, so you don't need to store sensitive information in the plan file itself**

## Input
## Running

To run jsoninator, you need to provide it with a plan file:

```bash
jsoninator -plan=my-plan.yaml
```

By default, jsoninator will not write to any outputs and instead will perform a dry run, where changes that would be made will be shown.

To run jsoninator with consequences, pass `-dryrun=false`, eg:

```bash
jsoninator -plan=my-plan.yaml -dryrun=false
```


Input configuration is limited to `http` and `raw`. 

### HTTP

Configure an HTTP input source with a `url`. Optionally provide a `headers` map that will be sent along with the request. 

Input HTTP requests are always HTTP GETs.

The resulting JSON should be either a JSON object or a JSON array of objects. Other formats are not supported.

## Pipeline

Pipeline processors process input items individually (either the single object from the input or each item in the JSON array from the input), in order. Any processor that returns `nil` will stop processing for that message.

There are several processors:

### Filter

The filter processor excludes messages.

Filters can be configured to match in 3 ways (they can be used in any combination):

#### Prefix

`prefix` takes a map of field name to prefixes. **ALL** entries in the map must match for an item to pass the check. **Prefixes are case sensitive**.

This pipeline will only return objects whose `name` starts with `bo` and whose city starts with `Los`.

```yaml
pipeline:
  processors:
    - filter:
        prefix:
          name: bo
          city: Los
```

#### Suffix

`suffix` takes a map of field name to suffixes. **ALL** entries in the map must match for an item to pass the check. **Suffixes are case sensitive**.

This pipeline will only return objects whose `name` ends with `ert` and whose company ends with `LLC`.

```yaml
pipeline:
  processors:
    - filter:
        suffix:
          name: ert
          company: LLC
```

#### Query

`query` allows [Go template](https://pkg.go.dev/text/template) evaluation of the message. If the resulting text is `true`, the message matches the filter. Any other value (including blank) will not match.

This pipeline will only return objects who are older than 65:

**Note: ALL numbers are considered floats in templates. If you get an error about incompatible types for comparison for an int, the simplest fix is to add a `.0` to the end of the magic number.**

```yaml
pipeline:
  processors:
    - filter:
        query: |
          {{if gt .age 65.0}}true{{else}}false{{end}}
```

### Transform

The transform processor allows modifications to individual fields in an object. 

This can be used to provide default values, add values where they're missing, omit values, or change values indiscriminately. The transform `fields` is a map of field names to templates. The templates will be evaluated with the message as the data context.

If a template evaluates to the string `nil`, the value will be removed from the object. 

Values have their surrounding whitespace removed after evaluation, so you can be generous with spaces in the plan YAML.

```yaml
pipeline:
  processors:
    - transform:
        fields:
          alwaysOverwrite: this value is always overwritten
          deleted: |
            {{ if eq .deleted "deleteme" }}nil{{else}}{{.deleted}}{{end}}
          defaultValue: |
            {{ if .defaultValue }}
              {{ .defaultValue }}
            {{else}}
                default value
            {{end}}
```

With the following input

```json
[
    {
        "alwaysOverwrite": "something",
        "deleted": "no",
        "defaultValue": "is set"
    },
    {
        "deleted": "deleteme",
    }
]
```

The output would be

```json
[
    {
        "alwaysOverwrite": "this value is always overwritten",
        "deleted": "no",
        "defaultValue": "is set"
    },
    {
        "alwaysOverwrite": "this value is always overwritten",
        "defaultValue": "default value"
    }
]
```

### Map

The map processor selects a field or nested field inside the object. Subsequent pipeline processors will work with the narrowed objects.

If a `default` is provided, if a value isn't found at the field location, it will be emitted in its place.

Given this pipeline:

```yaml
pipeline:
  processors:
    - map:
        field: location
        default:
          city: Austin
          state: Texas
```

And this input:

```json
[
    {
        "name": "Houstonian",
        "location": {
            "city": "Houston",
            "state": "Texas"
        }
    },
    {
        "name": "Austinite"
    }
]
```

The resulting output would be:

```json
[
    {
        "city": "Houston",
        "state": "Texas"
    },
    {
        "city": "Austin",
        "state": "Texas"
    }
]
```

### Replace

The replace processor emits an object with only the properties defined in the plan. Output values support templates, and will have the current message as the data context.

Given this pipeline:

```yaml
pipeline:
  processors:
    - replace:
        template:
          retired: |
            {{ if gt .age 64.9 }}true{{else}}false{{end}}
```

And this input:

```json
[
    {
        "name": "Retiree",
        "age": 70
    },
    {
        "name": "Middle aged",
        "age": 40
    }
]
```

The resulting output would be:

```json
[
    {
        "retired": true
    },
    {
        "retired": false
    }
]
```

## Output

Each message from the pipeline will be sent to the output. The only supported output channel is an HTTP endpoint.

The JSON of the message will be provided in the request body.

The output step requires a `url` and `method` as the destination URL and appropriate HTTP method, respectively.

The URL supports templates, and the data context is the **original** message. This allows mapping deep into an object but referencing its `id` later, for example.

If a `status_codes` array is provided, the plan will error out if the response status code isn't included. 

If `headers` are provided, they will be sent with the HTTP request.

## Reporting

There are 3 channels to watch for progress, errors, and auditing.

* `stdout` receives simple progress messages and status information.
* `stderr` receives log messages for troubleshooting and debugging
* A `reports` directory will be created wherever this is run. In it will be a datestamped folder with random numbers at the end, and 3 files: `changes.csv`, `filtered.csv`, and `noops.csv`.
* * `changes.csv` will record changes made through the transform processor with the message's id, the field changed, and its before and after values.
* * `filtered.csv` will have a list of messages that were filtered, and the filter that excluded them.
* * `noops.csv` will have a list of messages that were included in the entire pipeline but that had no changes made.

# Issues

Issues and pull requests are welcome. Please use GitHub issues to report a defect or request an improvement.