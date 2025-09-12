package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func yamlify(str string) []byte {
	leader := ""
	var out bytes.Buffer
	str = strings.ReplaceAll(str, "  ", "\t")
	for line := range strings.SplitSeq(str, "\n") {
		if len(line) == 0 {
			continue
		}
		if leader == "" {
			for range strings.Count(line, "\t") {
				leader += "\t"
			}
			//fmt.Fprintf(os.Stderr, "Leader: %q\n", leader)
		}
		lefted := strings.TrimRight(strings.ReplaceAll(strings.Replace(line, leader, "", 1), "\t", "  "), "\t ")
		//fmt.Fprintf(os.Stderr, "line:\n%q\n%q\n", line, lefted)
		if len(lefted) > 0 {
			out.WriteString(lefted + "\n")
		}
	}

	//fmt.Fprintf(os.Stderr, "YAML:\n%s\n", out.String())
	return out.Bytes()
}

func Test_Pipeline(t *testing.T) {
	go func() {
		for range reports {

		}
	}()

	t.Run("map", func(t *testing.T) {
		t.Run("happy path", func(t *testing.T) {
			processor := Map{
				Field: "config.gateway",
			}

			input := map[string]any{
				"config": map[string]any{
					"gateway": "gwgwgw",
				},
			}

			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := processor.Process(ctx, input)
			require.NoError(t, err)
			res, ok := output.(string)
			require.True(t, ok)
			require.Equal(t, "gwgwgw", res)
		})

		t.Run("default value", func(t *testing.T) {
			processor := Map{
				Field:   "config.gateway",
				Default: "123",
			}

			input := map[string]any{}
			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := processor.Process(ctx, input)
			require.NoError(t, err)
			require.Equal(t, "123", output)
		})

		t.Run("docs example", func(t *testing.T) {
			var pipeline Pipeline
			require.NoError(t, yaml.Unmarshal(yamlify(`
		processors:
			- map:
					field: location
					default:
						city: Austin
						state: Texas
				`), &pipeline))

			require.Len(t, pipeline.Processors, 1)
			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := pipeline.Process(ctx, map[string]any{
				"name": "Houstonian",
				"location": map[string]any{
					"city":  "Houston",
					"state": "Texas",
				},
			})
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Houston", res["city"])
			assert.Equal(t, "Texas", res["state"])

			output, err = pipeline.Process(ctx, map[string]any{"name": "Austinite"})
			require.NoError(t, err)
			res, ok = output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Austin", res["city"])
			assert.Equal(t, "Texas", res["state"])
		})
	})

	t.Run("filter", func(t *testing.T) {
		inputs := []map[string]any{
			{"name": "bamboozle", "protocol": "tcp", "port": 80, "city": "Los Angeles"},
			{"name": "bambi", "protocol": "udp", "port": 53, "city": "Boise"},
			{"name": "fizboozle", "protocol": "udp", "port": 123, "city": "Los Alamos"},
		}

		t.Run("suffix", func(t *testing.T) {
			t.Run("list happy path", func(t *testing.T) {
				processor := Filter{
					Suffix: map[string]string{
						"name": "oozle",
					},
				}

				for _, input := range inputs {
					t.Run("filtering suffix"+fmt.Sprint(input), func(t *testing.T) {
						ctx, cancel := WithReporter(t.Context(), "test")
						defer cancel()
						output, err := processor.Process(ctx, input)
						require.NoError(t, err)
						if strings.HasSuffix(input["name"].(string), "oozle") {
							res := output.(map[string]any)
							assert.True(t, strings.HasSuffix(res["name"].(string), "oozle"))
						} else {
							require.Nil(t, output)
						}
					})
				}
			})

		})
		t.Run("prefix", func(t *testing.T) {
			t.Run("list happy path", func(t *testing.T) {
				processor := Filter{
					Prefix: map[string]string{
						"name": "bam",
					},
				}

				for _, input := range inputs {
					t.Run("filtering prefix"+fmt.Sprint(input), func(t *testing.T) {
						ctx, cancel := WithReporter(t.Context(), "test")
						defer cancel()
						output, err := processor.Process(ctx, input)
						require.NoError(t, err)
						if input["name"].(string)[0:3] == "bam" {
							res := output.(map[string]any)
							assert.True(t, strings.HasPrefix(res["name"].(string), "bam"))
						} else {
							require.Nil(t, output)
						}
					})
				}
			})

			t.Run("bam Los prefixes", func(t *testing.T) {
				var pipeline Pipeline
				require.NoError(t, yaml.Unmarshal(yamlify(`
				processors:
					- filter:
							prefix:
								name: bam
								city: Los
				`), &pipeline))

				for _, input := range inputs {
					t.Run("filtering prefix"+fmt.Sprint(input), func(t *testing.T) {
						ctx, cancel := WithReporter(t.Context(), "test")
						defer cancel()
						output, err := pipeline.Process(ctx, input)
						require.NoError(t, err)
						if input["name"].(string)[0:3] == "bam" && strings.HasPrefix(input["city"].(string), "Los") {
							res := output.(map[string]any)
							assert.True(t, strings.HasPrefix(res["name"].(string), "bam"))
							assert.True(t, strings.HasPrefix(res["city"].(string), "Los"))
						} else {
							require.Nil(t, output)
						}
					})
				}
			})

		})
		t.Run("query", func(t *testing.T) {
			t.Run("list happy path", func(t *testing.T) {
				processor := Filter{
					Query: `{{if hasPrefix .protocol "udp"}}true{{end}}`,
				}

				for _, input := range inputs {
					t.Run("filtering "+fmt.Sprint(input), func(t *testing.T) {
						ctx, cancel := WithReporter(t.Context(), "test")
						defer cancel()
						output, err := processor.Process(ctx, input)
						require.NoError(t, err)
						if input["protocol"] == "udp" {
							res, ok := output.(map[string]any)
							require.True(t, ok)
							require.Equal(t, "udp", res["protocol"])
							require.Equal(t, input["port"], res["port"])
						} else {
							require.Nil(t, output)
						}
					})
				}
			})

			t.Run("age check", func(t *testing.T) {
				var pipeline Pipeline
				require.NoError(t, yaml.Unmarshal(yamlify(`
				processors:
					- filter:
							query: |
								{{if (gt .age 65.0)}}true{{end}}
				`), &pipeline))

				ctx, cancel := WithReporter(t.Context(), "test")
				defer cancel()

				msg, err := pipeline.Process(ctx, map[string]any{"age": 70})
				require.NoError(t, err)
				require.NotNil(t, msg)

				msg, err = pipeline.Process(ctx, map[string]any{"age": 20})
				require.NoError(t, err)
				require.Nil(t, msg)
			})
		})
	})

	t.Run("replace", func(t *testing.T) {
		t.Run("happy path", func(t *testing.T) {
			processor := Replace{
				Template: map[string]string{"hi": "five"},
			}

			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := processor.Process(ctx, []byte(`{"foo":"bar"}`))
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			require.Equal(t, "five", res["hi"])
		})

		t.Run("sourcing data from input", func(t *testing.T) {
			processor := Replace{
				Template: map[string]string{"hi": "{{.foo}}"},
			}

			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := processor.Process(ctx, map[string]any{"foo": "bar"})
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			require.Equal(t, "bar", res["hi"])
		})

		t.Run("docs example", func(t *testing.T) {
			var pipeline Pipeline
			require.NoError(t, yaml.Unmarshal(yamlify(`
		processors:
			- replace:
					template:
						retired: |
            				{{ if gt .age 65.0 }}true{{else}}false{{end}}
						another: true
						number: 123
				`), &pipeline))

			require.Len(t, pipeline.Processors, 1)
			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := pipeline.Process(ctx, map[string]any{
				"name": "Retiree",
				"age":  70,
			})
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, true, res["retired"])
			assert.Equal(t, true, res["another"])
			assert.Equal(t, float64(123), res["number"])

			output, err = pipeline.Process(ctx, map[string]any{
				"name": "Middle aged",
				"age":  40,
			})
			require.NoError(t, err)
			res, ok = output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, false, res["retired"])
			assert.Equal(t, true, res["another"])
			assert.Equal(t, float64(123), res["number"])
		})
	})

	t.Run("transform", func(t *testing.T) {
		t.Run("happy path", func(t *testing.T) {
			processor := Transform{
				Fields: map[string]string{
					"new_field": "static value",
					"foo_field": "{{.foo}}",
				},
			}

			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := processor.Process(ctx, map[string]any{"foo": "bar"})
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			require.Equal(t, "static value", res["new_field"])
			require.Equal(t, "bar", res["foo_field"])
		})

		t.Run("removes undefined", func(t *testing.T) {
			processor := Transform{
				Fields: map[string]string{
					"undef": "nil",
				},
			}

			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := processor.Process(ctx, map[string]any{"foo": "bar"})
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "bar", res["foo"])
			_, exists := res["undef"]
			require.False(t, exists)
		})

		t.Run("docs example", func(t *testing.T) {
			var pipeline Pipeline
			require.NoError(t, yaml.Unmarshal(yamlify(`
		processors:
			- transform:
					fields:
			  			alwaysOverwrite: this value is always overwritten
 						# delete the value from the resulting object if its value is "deleteme"
			  			deleted: |
								{{ if eq .deleted "deleteme" }}nil{{else}}{{.deleted}}{{end}}
			  			defaultValue: |
			    				{{ if .defaultValue }}
			      				{{ .defaultValue }}
			    				{{else}}
			        			default value
			    				{{end}}
				`), &pipeline))

			require.Len(t, pipeline.Processors, 1)
			ctx, cancel := WithReporter(t.Context(), "test")
			defer cancel()
			output, err := pipeline.Process(ctx, map[string]any{
				"alwaysOverwrite": "something",
				"deleted":         "no",
				"defaultValue":    "is set",
			})
			require.NoError(t, err)
			res, ok := output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "this value is always overwritten", res["alwaysOverwrite"])
			assert.Equal(t, "no", res["deleted"])
			assert.Equal(t, "is set", res["defaultValue"])

			output, err = pipeline.Process(ctx, map[string]any{"deleted": "deleteme"})
			require.NoError(t, err)
			res, ok = output.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "this value is always overwritten", res["alwaysOverwrite"])
			_, exists := res["deleted"]
			require.False(t, exists)
			assert.Equal(t, "default value", res["defaultValue"])
		})
	})

	t.Run("e2e", func(t *testing.T) {
		t.Run("tg udp case", func(t *testing.T) {
			pipeline := Pipeline{
				Processors: []Processor{
					Transform{
						Fields: map[string]string{
							"udpEnabled":         "true",
							"udpPort":            "{{if .udpPort}}{{.udpPort}}{{else}}{{.port}}{{end}}",
							"maxClientWriteMbps": "{{if eq .maxClientWriteMbps 0.0}}nil{{else}}{{.maxClientWriteMbps}}{{end}}",
						},
					},
				},
			}

			type config struct {
				Name          string `json:"-"`
				Enabled       bool   `json:"enabled"`
				MaxMBPS       int    `json:"maxmbps"`
				Cert          string `json:"cert"`
				Type          string `json:"type"`
				UDPPort       int    `json:"udpPort"`
				HopMonitoring struct {
					MonitorHops string `json:"monitorHops"`
				} `json:"hopMonitoring,omitempty"`
				UDPEnabled                 bool    `json:"udpEnabled"`
				MaxClientWriteMBPS         int     `json:"maxClientWriteMbps"`
				Port                       int     `json:"port"`
				ExpectedUDPPort            float64 `json:"-"`
				ExpectedMaxClientWriteMBPS float64 `json:"-"`
			}

			tests := []config{
				{
					Name:                       "simple",
					MaxMBPS:                    100,
					Cert:                       "example.com",
					Type:                       "private",
					Enabled:                    false,
					Port:                       8993,
					ExpectedUDPPort:            8993,
					ExpectedMaxClientWriteMBPS: -1,
				},
			}

			for _, test := range tests {
				t.Run(test.Name, func(t *testing.T) {
					var input any
					b, err := json.Marshal(test)
					require.NoError(t, err)
					err = json.Unmarshal(b, &input)
					require.NoError(t, err)

					ctx, cancel := WithReporter(t.Context(), "test")
					defer cancel()
					out, err := pipeline.Process(ctx, input)
					require.NoError(t, err)
					m, ok := out.(map[string]any)
					require.True(t, ok)

					// Passthru fields
					assert.Equal(t, test.Enabled, m["enabled"])
					assert.Equal(t, float64(test.MaxMBPS), m["maxmbps"])
					assert.Equal(t, test.Cert, m["cert"])
					assert.Equal(t, test.Type, m["type"])
					assert.Equal(t, float64(test.Port), m["port"])
					hm, ok := m["hopMonitoring"].(map[string]any)
					require.True(t, ok)
					assert.Equal(t, test.HopMonitoring.MonitorHops, hm["monitorHops"])

					// Modified fields
					assert.Equal(t, true, m["udpEnabled"])
					assert.Equal(t, test.ExpectedUDPPort, m["udpPort"])
					if test.ExpectedMaxClientWriteMBPS == -1 {
						_, exists := m["maxClientWriteMbps"]
						assert.False(t, exists)
					} else {
						assert.Equal(t, test.ExpectedMaxClientWriteMBPS, m["maxClientWriteMbps"])
					}
				})
			}
		})
	})
}
