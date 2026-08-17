package tools

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestControlRecording(t *testing.T) {
	tests := []struct {
		name       string
		action     RecordAction
		lookPathOk bool
		runErr     error
		runOut     string
		wantErr    bool
		wantArgs   []string
	}{
		{
			name:       "toggle success",
			action:     RecordActionToggle,
			lookPathOk: true,
			wantArgs:   []string{"voxtype", "record", "toggle"},
		},
		{
			name:       "start success",
			action:     RecordActionStart,
			lookPathOk: true,
			wantArgs:   []string{"voxtype", "record", "start"},
		},
		{
			name:       "stop success",
			action:     RecordActionStop,
			lookPathOk: true,
			wantArgs:   []string{"voxtype", "record", "stop"},
		},
		{
			name:       "voxtype not found",
			action:     RecordActionToggle,
			lookPathOk: false,
			wantErr:    true,
		},
		{
			name:       "run failure",
			action:     RecordActionStart,
			lookPathOk: true,
			runErr:     errors.New("daemon crashed"),
			runOut:     "failed to connect to socket",
			wantErr:    true,
			wantArgs:   []string{"voxtype", "record", "start"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recordedArgs []string
			d := Dependencies{
				Stdout: &bytes.Buffer{},
				LookPath: func(file string) (string, error) {
					if file == "voxtype" && tt.lookPathOk {
						return "/usr/bin/voxtype", nil
					}
					return "", errors.New("not found")
				},
				Run: func(ctx context.Context, name string, args ...string) error {
					recordedArgs = append([]string{name}, args...)
					return tt.runErr
				},
			}

			err := ControlRecording(context.Background(), d, tt.action)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ControlRecording() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if len(tt.wantArgs) > 0 {
				if strings.Join(recordedArgs, " ") != strings.Join(tt.wantArgs, " ") {
					t.Errorf("got args %v, want %v", recordedArgs, tt.wantArgs)
				}
			}
		})
	}
}

func TestGetRecordingStatus(t *testing.T) {
	tests := []struct {
		name       string
		lookPathOk bool
		runOut     string
		runErr     error
		wantStatus string
		wantErr    bool
	}{
		{
			name:       "idle status",
			lookPathOk: true,
			runOut:     "idle\n",
			wantStatus: "idle",
		},
		{
			name:       "recording status",
			lookPathOk: true,
			runOut:     "recording\n",
			wantStatus: "recording",
		},
		{
			name:       "listening status",
			lookPathOk: true,
			runOut:     "Status: listening for speech...\n",
			wantStatus: "recording",
		},
		{
			name:       "voxtype not found",
			lookPathOk: false,
			wantStatus: "inactive",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Dependencies{
				Stdout: &bytes.Buffer{},
				LookPath: func(file string) (string, error) {
					if file == "voxtype" && tt.lookPathOk {
						return "/usr/bin/voxtype", nil
					}
					return "", errors.New("not found")
				},
				RunOutput: func(ctx context.Context, name string, args ...string) (string, error) {
					return tt.runOut, tt.runErr
				},
			}

			stat, err := GetRecordingStatus(context.Background(), d)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetRecordingStatus() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if stat != tt.wantStatus {
				t.Errorf("got status %q, want %q", stat, tt.wantStatus)
			}
		})
	}
}
