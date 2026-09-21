package telemetry

import "fmt"

type QualityResult struct {
	Name, Description  string
	Checked, Offending int64
	Percentage         float64
	Warn               bool
}

func (r QualityResult) Status() string {
	if r.Warn {
		return "WARN"
	}
	return "PASS"
}

func (d *DB) QualityChecks() ([]QualityResult, error) {
	spec := mustTelemetrySQL()
	out := make([]QualityResult, 0, len(spec.QualityChecks))
	for _, check := range spec.QualityChecks {
		var r QualityResult
		r.Name, r.Description = check.Name, check.Description
		if err := d.sql.QueryRow(check.SQL).Scan(&r.Checked, &r.Offending, &r.Percentage); err != nil {
			return nil, fmt.Errorf("telemetry quality check %q: %w", check.Name, err)
		}
		r.Warn = r.Checked > 0 && r.Percentage > check.WarnAbovePercent
		out = append(out, r)
	}
	return out, nil
}
