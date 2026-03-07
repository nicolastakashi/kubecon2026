package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type registry struct {
	Groups []group `yaml:"groups"`
}

type group struct {
	ID         string      `yaml:"id"`
	Type       string      `yaml:"type"`
	Brief      string      `yaml:"brief,omitempty"`
	Stability  string      `yaml:"stability,omitempty"`
	Attributes []attribute `yaml:"attributes,omitempty"`
	MetricName string      `yaml:"metric_name,omitempty"`
	Instrument string      `yaml:"instrument,omitempty"`
	Unit       string      `yaml:"unit,omitempty"`
}

type attribute struct {
	ID               string      `yaml:"id"`
	Type             string      `yaml:"type,omitempty"`
	Brief            string      `yaml:"brief,omitempty"`
	Examples         interface{} `yaml:"examples,omitempty"`
	RequirementLevel string      `yaml:"requirement_level,omitempty"`
	Note             string      `yaml:"note,omitempty"`
	Stability        string      `yaml:"stability,omitempty"`
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

	filtered := make([]group, 0, len(doc.Groups))
	for _, g := range doc.Groups {
		if g.Type != "metric" {
			continue
		}
		if isRuntimeMetric(g.MetricName) {
			continue
		}

		slices.SortFunc(g.Attributes, func(a, b attribute) int {
			return strings.Compare(a.ID, b.ID)
		})
		filtered = append(filtered, g)
	}

	slices.SortFunc(filtered, func(a, b group) int {
		return strings.Compare(a.ID, b.ID)
	})

	out, err := yaml.Marshal(registry{Groups: filtered})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal registry: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "write registry: %v\n", err)
		os.Exit(1)
	}
}

func isRuntimeMetric(name string) bool {
	return strings.HasPrefix(name, "go_") ||
		strings.HasPrefix(name, "process_") ||
		strings.HasPrefix(name, "scrape_") ||
		strings.HasPrefix(name, "promhttp_") ||
		name == "up"
}
