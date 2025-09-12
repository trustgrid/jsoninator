package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	_ "embed"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/nodes.json
var nodesjson string

var e2eYAML = fmt.Sprintf(`
input:
  raw: |
    %s

pipeline:
  processors:
    - filter:
        query: |
          {{if and (eq .type "Node") (eq "false" .tags.Flagged)}}true{{else}}false{{end}}
    - map:
        field: config.gateway
    - transform:
        fields:
          udpEnabled: true
`, strings.ReplaceAll(nodesjson, "\n", ""))

func Test_Plan(t *testing.T) {
	go func() {
		select {
		case <-reports:
		case <-t.Context().Done():
			return
		}
	}()

	t.Run("parsing", func(t *testing.T) {
		t.Run("happy path", func(t *testing.T) {
			yamlData := `
input:
  http:
    url: http://example.com
    headers:
      Authorization: something something
      Alpha: beta

`

			plan, err := Parse([]byte(yamlData))
			require.NoError(t, err)
			require.Equal(t, "http://example.com", plan.Input.HTTP.URL)
			require.Equal(t, map[string]string{
				"Authorization": "something something",
				"Alpha":         "beta",
			}, plan.Input.HTTP.Headers)
		})
	})

	t.Run("injects environment variables", func(t *testing.T) {
		yamlData := `
input:
  http:
    url: http://example.com
    headers:
      Authorization: ${AUTHORIZATION}
      Alpha: beta
`
		t.Setenv("AUTHORIZATION", "something something")

		plan, err := Parse([]byte(yamlData))
		require.NoError(t, err)
		require.Equal(t, "http://example.com", plan.Input.HTTP.URL)
		require.Equal(t, map[string]string{
			"Authorization": "something something",
			"Alpha":         "beta",
		}, plan.Input.HTTP.Headers)
	})

	t.Run("e2e", func(t *testing.T) {
		type Config struct {
			Enabled            bool `json:"enabled"`
			UDPEnabled         bool `json:"udpEnabled"`
			Port               int  `json:"port"`
			UDPPort            int  `json:"udpPort"`
			MaxClientWriteMBPS *int `json:"maxClientWriteMbps,omitempty"`
			MaxMBPS            int  `json:"maxmbps"`
		}

		plan, err := Parse([]byte(e2eYAML))
		require.NoError(t, err)
		plan.skipReporter = true
		buf := bytes.NewBuffer(nil)
		plan.Output.Buffer = buf
		require.NoError(t, plan.Run(t.Context()))
		var outputs []Config
		for _, line := range strings.Split(buf.String(), "\n") {
			if line == "" {
				continue
			}
			var c Config
			require.NoError(t, json.Unmarshal([]byte(line), &c))
			outputs = append(outputs, c)
		}

		for _, o := range outputs {
			assert.True(t, o.UDPEnabled)
			assert.Equal(t, o.Port, o.UDPPort)
			if o.MaxClientWriteMBPS != nil {
				assert.NotZero(t, *o.MaxClientWriteMBPS)
			}
		}
	})
}
