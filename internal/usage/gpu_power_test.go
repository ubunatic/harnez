package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGPUTempPowerPart(t *testing.T) {
	for name, tc := range map[string]struct {
		g    GPU
		want string
	}{
		"both":       {GPU{TempC: 52.4, HaveTemp: true, PowerW: 15.38, HavePower: true}, " (52°C/15W)"},
		"temp only":  {GPU{TempC: 52.4, HaveTemp: true}, " (52°C)"},
		"power only": {GPU{PowerW: 8, HavePower: true}, " (8W)"},
		"neither":    {GPU{}, ""},
	} {
		if got := gpuTempPowerPart(tc.g); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestReadAMDGPUPower(t *testing.T) {
	write := func(t *testing.T, dir, name, val string) {
		t.Helper()
		hw := filepath.Join(dir, "hwmon", "hwmon3")
		if err := os.MkdirAll(hw, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(hw, name), []byte(val+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	avg := t.TempDir()
	write(t, avg, "power1_average", "6948000")
	write(t, avg, "power1_input", "15383000")
	if w, ok := readAMDGPUPower(avg); !ok || w != 6.948 {
		t.Errorf("average preferred: got %v/%v, want 6.948", w, ok)
	}

	input := t.TempDir()
	write(t, input, "power1_input", "15383000")
	if w, ok := readAMDGPUPower(input); !ok || w != 15.383 {
		t.Errorf("input fallback: got %v/%v, want 15.383", w, ok)
	}

	garbage := t.TempDir()
	write(t, garbage, "power1_average", "not a number")
	write(t, garbage, "power1_input", "15383000")
	if w, ok := readAMDGPUPower(garbage); !ok || w != 15.383 {
		t.Errorf("garbage average fallback: got %v/%v, want 15.383", w, ok)
	}

	if _, ok := readAMDGPUPower(t.TempDir()); ok {
		t.Error("no hwmon power files: ok = true, want false")
	}
}
