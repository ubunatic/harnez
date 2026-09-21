package telemetry

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"sync"
	"ubunatic.com/harnez"
)

type telemetrySQLSpec struct {
	Schema     string            `yaml:"schema"`
	Statements map[string]string `yaml:"statements"`
	Predicates map[string]struct {
		SQL string `yaml:"sql"`
	} `yaml:"predicates"`
	GroupColumns  []string           `yaml:"group_columns"`
	QualityChecks []qualityCheckSpec `yaml:"quality_checks"`
}

type qualityCheckSpec struct {
	Name             string  `yaml:"name"`
	SQL              string  `yaml:"sql"`
	Description      string  `yaml:"description"`
	WarnCondition    string  `yaml:"warn_condition"`
	WarnAbovePercent float64 `yaml:"warn_above_percent"`
	Result           struct {
		Checked    string `yaml:"checked"`
		Offending  string `yaml:"offending"`
		Percentage string `yaml:"percentage"`
	} `yaml:"result"`
}

func (s *telemetrySQLSpec) groupColumnAllowed(column string) bool {
	for _, allowed := range s.GroupColumns {
		if allowed == column {
			return true
		}
	}
	return false
}

func loadTelemetrySQL() (*telemetrySQLSpec, error) {
	b, e := fs.ReadFile(harnez.DefaultFS, "spec/telemetry.yaml")
	if e != nil {
		return nil, e
	}
	var s telemetrySQLSpec
	if e = yaml.Unmarshal(b, &s); e != nil {
		return nil, e
	}
	if s.Schema == "" || len(s.Statements) == 0 {
		return nil, fmt.Errorf("missing schema or statements")
	}
	for k, v := range s.Statements {
		if v == "" {
			return nil, fmt.Errorf("empty statement %q", k)
		}
	}
	for _, q := range s.QualityChecks {
		if q.Name == "" || q.SQL == "" || q.Result.Checked == "" || q.Result.Offending == "" || q.Result.Percentage == "" {
			return nil, fmt.Errorf("incomplete quality check %q", q.Name)
		}
	}
	return &s, nil
}

var telemetrySQLOnce = sync.OnceValues(loadTelemetrySQL)

func mustTelemetrySQL() *telemetrySQLSpec {
	s, e := telemetrySQLOnce()
	if e != nil {
		panic(fmt.Sprintf("harnez telemetry spec: %v", e))
	}
	return s
}
