package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top level structure loaded from a YAML file.
type Config struct {
	// Timeout applied to every request unless overridden on the CLI.
	Timeout Duration `yaml:"timeout"`
	// Interval is how often watch mode re-runs the checks.
	Interval  Duration   `yaml:"interval"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

// Endpoint describes a single target to probe.
type Endpoint struct {
	Name         string            `yaml:"name"`
	URL          string            `yaml:"url"`
	Method       string            `yaml:"method"`
	ExpectStatus int               `yaml:"expect_status"`
	Headers      map[string]string `yaml:"headers"`
}

// Method returns the HTTP method, defaulting to GET.
func (e Endpoint) HTTPMethod() string {
	if e.Method == "" {
		return "GET"
	}
	return e.Method
}

// WantStatus returns the status code we treat as healthy, defaulting to 200.
func (e Endpoint) WantStatus() int {
	if e.ExpectStatus == 0 {
		return 200
	}
	return e.ExpectStatus
}

// Load reads and parses a config file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if cfg.Timeout.Duration == 0 {
		cfg.Timeout.Duration = 5 * time.Second
	}
	if cfg.Interval.Duration == 0 {
		cfg.Interval.Duration = 10 * time.Second
	}

	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("%s: no endpoints defined", path)
	}
	for i, ep := range cfg.Endpoints {
		if ep.URL == "" {
			return nil, fmt.Errorf("%s: endpoint %d is missing a url", path, i+1)
		}
		if ep.Name == "" {
			cfg.Endpoints[i].Name = ep.URL
		}
	}

	return &cfg, nil
}

// Duration wraps time.Duration so it can be unmarshalled from a YAML string
// like "5s" or "2m" instead of a raw nanosecond count.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}
