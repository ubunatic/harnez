package sound

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// specPath is the embedded sound player fallback registry; see docs/Spec.md.
const specPath = "spec/sound.yaml"

// PlayerSpec defines a single audio player command and arguments.
type PlayerSpec struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
}

// PlatformSpec defines the sound file and player chain for an operating system.
type PlatformSpec struct {
	SoundFile string       `yaml:"sound_file"`
	Players   []PlayerSpec `yaml:"players"`
}

// Spec is the top-level shape of spec/sound.yaml.
type Spec struct {
	Timeout   string                  `yaml:"timeout"`
	Platforms map[string]PlatformSpec `yaml:"platforms"`
}

// TimeoutDuration parses the timeout string into a time.Duration.
func (s *Spec) TimeoutDuration() (time.Duration, error) {
	if s == nil || s.Timeout == "" {
		return 0, fmt.Errorf("missing sound playback timeout")
	}
	d, err := time.ParseDuration(s.Timeout)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid timeout %q: %w", s.Timeout, err)
	}
	return d, nil
}

// ForOS returns the PlatformSpec for the requested operating system name.
func (s *Spec) ForOS(goos string) (*PlatformSpec, error) {
	if s == nil || len(s.Platforms) == 0 {
		return nil, fmt.Errorf("no platform sound configurations loaded")
	}
	p, ok := s.Platforms[goos]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedOS, goos)
	}
	return &p, nil
}

// LoadSpec reads and validates the embedded sound specification.
func LoadSpec() (*Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", specPath, err)
	}
	return ParseSpec(data)
}

// ParseSpec unmarshals and validates sound specification YAML bytes.
func ParseSpec(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", specPath, err)
	}
	if d, err := time.ParseDuration(s.Timeout); err != nil || d <= 0 {
		return nil, fmt.Errorf("%s: timeout %q is not a positive duration", specPath, s.Timeout)
	}
	if len(s.Platforms) == 0 {
		return nil, fmt.Errorf("%s: platforms map is empty", specPath)
	}
	for osName, platform := range s.Platforms {
		if strings.TrimSpace(platform.SoundFile) == "" {
			return nil, fmt.Errorf("%s: platform %q: sound_file is empty", specPath, osName)
		}
		if len(platform.Players) == 0 {
			return nil, fmt.Errorf("%s: platform %q: players list is empty", specPath, osName)
		}
		for i, player := range platform.Players {
			if strings.TrimSpace(player.Name) == "" {
				return nil, fmt.Errorf("%s: platform %q player #%d: name is empty", specPath, osName, i)
			}
			if len(player.Command) == 0 {
				return nil, fmt.Errorf("%s: platform %q player %q: command is empty", specPath, osName, player.Name)
			}
			for j, cmdToken := range player.Command {
				if strings.TrimSpace(cmdToken) == "" {
					return nil, fmt.Errorf("%s: platform %q player %q: command token #%d is empty", specPath, osName, player.Name, j)
				}
			}
		}
	}
	return &s, nil
}
