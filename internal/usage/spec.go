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
	AGYMeterMaxAge string `yaml:"agy_meter_max_age"`
}

var (
	usageSpecOnce sync.Once
	usageSpecAge  time.Duration
	usageSpecErr  error
)

func agyMeterMaxAge() (time.Duration, error) {
	usageSpecOnce.Do(func() {
		data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
		if err != nil {
			usageSpecErr = fmt.Errorf("read embedded %s: %w", usageSpecPath, err)
			return
		}
		usageSpecAge, usageSpecErr = parseUsageSpec(data)
	})
	return usageSpecAge, usageSpecErr
}

func parseUsageSpec(data []byte) (time.Duration, error) {
	var spec usageSpec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return 0, fmt.Errorf("parse embedded %s: %w", usageSpecPath, err)
	}
	age, err := time.ParseDuration(spec.AGYMeterMaxAge)
	if err != nil || age <= 0 {
		return 0, fmt.Errorf("%s: agy_meter_max_age must be a positive duration", usageSpecPath)
	}
	return age, nil
}
