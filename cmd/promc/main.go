package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/xeipuuv/gojsonschema"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// MetricConfig represents the YAML configuration file structure.
type MetricConfig struct {
	Metrics     []Metric `yaml:"metrics"`
	PackageName string   `yaml:"package_name"`
}

type Metric struct {
	Name    string    `yaml:"name"`
	Type    string    `yaml:"type"`
	Labels  []string  `yaml:"labels,omitempty"`
	Help    string    `yaml:"help,omitempty"`
	Buckets []float64 `yaml:"buckets,omitempty"`
}

// Convert snake_case to CamelCase
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	c := cases.Title(language.English)
	for i, part := range parts {
		parts[i] = c.String(part)
	}
	return strings.Join(parts, "")
}

const metricsTemplate = `// Code generated by go generate; DO NOT EDIT.
package {{.PackageName}}

import (
    "github.com/prometheus/client_golang/prometheus"
)

func init() {
	// Automatically register metrics with Prometheus's default registry.
	{{range .Metrics}}
		prometheus.MustRegister({{snakeToCamel .Name}})
	{{- end}}
}

{{range .Metrics}}
	{{- if .Labels}}
		type {{snakeToCamel .Name}}Labels struct {
			{{- range .Labels}}
			{{snakeToCamel .}} string
			{{- end}}
		}
	{{- end}}

	{{- if eq .Type "counter"}}
		{{- if .Labels}}
			var {{snakeToCamel .Name}} = prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "{{.Name}}",
					Help: "{{.Help}}",
				},
				[]string{ {{- range .Labels}}"{{.}}",{{- end}} },
			)

			func Inc{{snakeToCamel .Name}}(labels {{snakeToCamel .Name}}Labels) {
				{{snakeToCamel .Name}}.With(prometheus.Labels{
					{{- range .Labels}}
					"{{.}}": labels.{{snakeToCamel .}},
					{{- end}}
				}).Inc()
			}
		{{- else}}
			var {{snakeToCamel .Name}} = prometheus.NewCounter(
				prometheus.CounterOpts{
					Name: "{{.Name}}",
					Help: "{{.Help}}",
				},
			)

			func Inc{{snakeToCamel .Name}}() {
				{{snakeToCamel .Name}}.Inc()
			}
		{{- end}}

	{{- else if eq .Type "gauge"}}
		{{- if .Labels}}
			var {{snakeToCamel .Name}} = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: "{{.Name}}",
					Help: "{{.Help}}",
				},
				[]string{ {{- range .Labels}}"{{.}}",{{- end}} },
			)

			func Set{{snakeToCamel .Name}}(labels {{snakeToCamel .Name}}Labels, value float64) {
				{{snakeToCamel .Name}}.With(prometheus.Labels{
					{{- range .Labels}}
					"{{.}}": labels.{{snakeToCamel .}},
					{{- end}}
				}).Set(value)
			}
		{{- else}}
			var {{snakeToCamel .Name}} = prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "{{.Name}}",
					Help: "{{.Help}}",
				},
			)

			func Set{{snakeToCamel .Name}}(value float64) {
				{{snakeToCamel .Name}}.Set(value)
			}
		{{- end}}
	{{- else if eq .Type "histogram"}}
		{{- if .Labels}}
			var {{snakeToCamel .Name}} = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name: "{{.Name}}",
					Help: "{{.Help}}",
					Buckets: []float64{ {{- range .Buckets}}{{.}},{{- end}} },
				},
				[]string{ {{- range .Labels}}"{{.}}",{{- end}} },
			)

			func Observe{{snakeToCamel .Name}}(labels {{snakeToCamel .Name}}Labels, value float64) {
				{{snakeToCamel .Name}}.With(prometheus.Labels{
					{{- range .Labels}}
					"{{.}}": labels.{{snakeToCamel .}},
					{{- end}}
				}).Observe(value)
			}
		{{- else}}
			var {{snakeToCamel .Name}} = prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name: "{{.Name}}",
					Help: "{{.Help}}",
					Buckets: []float64{ {{- range .Buckets}}{{.}},{{- end}} },
				},
			)

			func Observe{{snakeToCamel .Name}}(value float64) {
				{{snakeToCamel .Name}}.Observe(value)
			}
		{{- end}}
	{{- end}}
{{- end}}
`

func main() {
	var configPath, outputPath, packageName string

	var rootCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generates Prometheus metrics based on a JSON configuration",
		Long: `A tool to generate Prometheus metrics Go code from a JSON configuration file.
Complete documentation is available at http://example.com`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load and parse the YAML configuration file.
			content, err := os.ReadFile(configPath)
			if err != nil {
				fmt.Printf("error reading config file: %v\n", err)
				os.Exit(1)
			}

			// Validate the JSON config
			err = validateConfig(content)
			if err != nil {
				fmt.Printf("config validation failed: %v\n", err)
				os.Exit(1)
			}

			var config MetricConfig
			err = json.Unmarshal(content, &config)
			if err != nil {
				fmt.Printf("error parsing config file: %v\n", err)
				os.Exit(1)
			}

			// Define a custom function map
			funcMap := template.FuncMap{
				"snakeToCamel": snakeToCamel,
			}

			// Generate Go code from the template with the custom function map.
			t, err := template.New("metrics").Funcs(funcMap).Parse(metricsTemplate)
			if err != nil {
				fmt.Printf("error parsing template: %v\n", err)
				os.Exit(1)
			}

			// Create a buffer to hold the executed template before formatting.
			var buf bytes.Buffer

			// Set package name in the config passed for template execution
			config.PackageName = packageName

			err = t.Execute(&buf, config)
			if err != nil {
				fmt.Printf("error executing template: %v\n", err)
				os.Exit(1)
			}

			// Format the source code in the buffer.
			formattedSource, err := format.Source(buf.Bytes())
			if err != nil {
				fmt.Printf("error formatting source: %v\n", err)
				os.Exit(1)
			}

			// Create the output file.
			outputFile, err := os.Create(outputPath)
			if err != nil {
				fmt.Printf("error creating output file: %v\n", err)
				os.Exit(1)
			}
			defer outputFile.Close()

			// Write the formatted source code to the output file.
			_, err = outputFile.Write(formattedSource)
			if err != nil {
				fmt.Printf("error writing to output file: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to the configuration file (required)")
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Path to the output file (required)")
	rootCmd.Flags().StringVarP(&packageName, "package", "p", "", "Package name for the output file (required)")

	rootCmd.MarkFlagRequired("config")
	rootCmd.MarkFlagRequired("output")
	rootCmd.MarkFlagRequired("package")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func validateConfig(content []byte) error {
	// Load the JSON schema
	schemaLoader := gojsonschema.NewStringLoader(metricConfigSchema)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return fmt.Errorf("error parsing schema: %v", err)
	}

	// Load the JSON config
	documentLoader := gojsonschema.NewBytesLoader(content)

	// Validate the JSON config against the schema
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("error validating config: %v", err)
	}

	if !result.Valid() {
		var errMessages []string
		for _, err := range result.Errors() {
			errMessages = append(errMessages, fmt.Sprintf("- %s", err))
		}
		return fmt.Errorf("invalid config:\n%s", strings.Join(errMessages, "\n"))
	}

	return nil
}