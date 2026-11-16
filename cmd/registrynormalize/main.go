package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// registry follows the Weaver `definition/2` file format.
type registry struct {
	Attributes []attribute `yaml:"attributes,omitempty"`
	Metrics    []metric    `yaml:"metrics"`
}

type attribute struct {
	Key       string      `yaml:"key"`
	Type      string      `yaml:"type,omitempty"`
	Brief     string      `yaml:"brief,omitempty"`
	Examples  interface{} `yaml:"examples,omitempty"`
	Note      string      `yaml:"note,omitempty"`
	Stability string      `yaml:"stability,omitempty"`
}

type metric struct {
	Name       string         `yaml:"name"`
	Instrument string         `yaml:"instrument,omitempty"`
	Unit       string         `yaml:"unit,omitempty"`
	Attributes []attributeRef `yaml:"attributes,omitempty"`
	Brief      string         `yaml:"brief,omitempty"`
	Stability  string         `yaml:"stability,omitempty"`
}

type attributeRef struct {
	Ref string `yaml:"ref"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: registrynormalize <registry.yaml>")
		os.Exit(2)
	}

	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read registry: %v\n", err)
		os.Exit(1)
	}

	var doc registry
	if err := yaml.Unmarshal(input, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal registry: %v\n", err)
		os.Exit(1)
	}

	out, err := yaml.Marshal(normalize(doc))
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal registry: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "write registry: %v\n", err)
		os.Exit(1)
	}
}

// normalize keeps application metrics and the attributes they reference, sorted, so diffs are stable.
func normalize(doc registry) registry {
	referenced := map[string]bool{}
	metrics := make([]metric, 0, len(doc.Metrics))
	for _, m := range doc.Metrics {
		if isRuntimeMetric(m.Name) {
			continue
		}
		slices.SortFunc(m.Attributes, func(a, b attributeRef) int {
			return strings.Compare(a.Ref, b.Ref)
		})
		for _, ref := range m.Attributes {
			referenced[ref.Ref] = true
		}
		metrics = append(metrics, m)
	}
	slices.SortFunc(metrics, func(a, b metric) int {
		return strings.Compare(a.Name, b.Name)
	})

	attributes := make([]attribute, 0, len(doc.Attributes))
	for _, a := range doc.Attributes {
		if referenced[a.Key] {
			attributes = append(attributes, a)
		}
	}
	slices.SortFunc(attributes, func(a, b attribute) int {
		return strings.Compare(a.Key, b.Key)
	})

	return registry{Attributes: attributes, Metrics: metrics}
}

func isRuntimeMetric(name string) bool {
	return strings.HasPrefix(name, "go_") ||
		strings.HasPrefix(name, "process_") ||
		strings.HasPrefix(name, "scrape_") ||
		strings.HasPrefix(name, "promhttp_") ||
		name == "up"
}
