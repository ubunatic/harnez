package release

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// PublishConfig contains the retry policy used for asset uploads.
type PublishConfig struct {
	UploadRetryAttempts int
	UploadRetryDelay    time.Duration
	ReleaseNotFound     *regexp.Regexp
}

type publishSpec struct {
	UploadRetryAttempts int    `yaml:"upload_retry_attempts"`
	UploadRetryDelay    string `yaml:"upload_retry_delay"`
	ReleaseNotFound     string `yaml:"release_not_found_pattern"`
}

func loadPublishConfig() (PublishConfig, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/release.yaml")
	if err != nil {
		return PublishConfig{}, fmt.Errorf("read spec/release.yaml: %w", err)
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/release.schema.json")
	if err != nil {
		return PublishConfig{}, fmt.Errorf("read release schema: %w", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return PublishConfig{}, fmt.Errorf("parse spec/release.yaml: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return PublishConfig{}, fmt.Errorf("parse release schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return PublishConfig{}, fmt.Errorf("resolve release schema: %w", err)
	}
	if err := resolved.Validate(document); err != nil {
		return PublishConfig{}, fmt.Errorf("validate spec/release.yaml: %w", err)
	}
	var spec publishSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return PublishConfig{}, fmt.Errorf("decode spec/release.yaml: %w", err)
	}
	delay, err := time.ParseDuration(spec.UploadRetryDelay)
	if err != nil {
		return PublishConfig{}, fmt.Errorf("spec/release.yaml: invalid upload_retry_delay: %w", err)
	}
	pattern, err := regexp.Compile(spec.ReleaseNotFound)
	if err != nil {
		return PublishConfig{}, fmt.Errorf("spec/release.yaml: invalid release_not_found_pattern: %w", err)
	}
	return PublishConfig{UploadRetryAttempts: spec.UploadRetryAttempts, UploadRetryDelay: delay, ReleaseNotFound: pattern}, nil
}
