package usage

import (
	"bytes"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const usageSpecPath = "spec/usage.yaml"

type usageSpec struct {
	AGYMeterMaxAge   string `yaml:"agy_meter_max_age"`
	CollectorCadence string `yaml:"collector_cadence"`
	CollectorTimeout string `yaml:"collector_timeout"`
}

var (
	usageSpecOnce    sync.Once
	usageSpecAge     time.Duration
	usageSpecCadence time.Duration
	usageSpecTimeout time.Duration
	usageSpecErr     error
)

func agyMeterMaxAge() (time.Duration, error) {
	age, _, _, err := usageDurations()
	return age, err
}

func collectorPolicy() (time.Duration, time.Duration, error) {
	_, cadence, timeout, err := usageDurations()
	return cadence, timeout, err
}

func usageDurations() (time.Duration, time.Duration, time.Duration, error) {
	usageSpecOnce.Do(func() {
		data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
		if err != nil {
			usageSpecErr = fmt.Errorf("read embedded %s: %w", usageSpecPath, err)
			return
		}
		usageSpecAge, usageSpecCadence, usageSpecTimeout, usageSpecErr = parseUsageSpec(data)
	})
	return usageSpecAge, usageSpecCadence, usageSpecTimeout, usageSpecErr
}

func parseUsageSpec(data []byte) (time.Duration, time.Duration, time.Duration, error) {
	var spec usageSpec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return 0, 0, 0, fmt.Errorf("parse embedded %s: %w", usageSpecPath, err)
	}
	age, err := time.ParseDuration(spec.AGYMeterMaxAge)
	if err != nil || age <= 0 {
		return 0, 0, 0, fmt.Errorf("%s: agy_meter_max_age must be a positive duration", usageSpecPath)
	}
	cadence, err := time.ParseDuration(spec.CollectorCadence)
	if err != nil || cadence <= 0 {
		return 0, 0, 0, fmt.Errorf("%s: collector_cadence must be a positive duration", usageSpecPath)
	}
	timeout, err := time.ParseDuration(spec.CollectorTimeout)
	if err != nil || timeout <= 0 {
		return 0, 0, 0, fmt.Errorf("%s: collector_timeout must be a positive duration", usageSpecPath)
	}
	return age, cadence, timeout, nil
}
