package tools

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{10 * 1024 * 1024, "10.0 MB"},
		{1536 * 1024 * 1024, "1.5 GB"},
	}
	for _, tc := range cases {
		if got := FormatBytes(tc.bytes); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestParseProcStatus(t *testing.T) {
	content := `Name:	harnez
Umask:	0022
State:	S (sleeping)
Tgid:	714116
Ngid:	0
Pid:	714116
PPid:	1658
TracerPid:	0
Uid:	1000	1000	1000	1000
Gid:	1000	1000	1000	1000
FDSize:	64
Groups:	10 1000
VmPeak:	 2303976 kB
VmSize:	 2303976 kB
VmLck:	       0 kB
VmPin:	       0 kB
VmHWM:	  202356 kB
VmRSS:	   15984 kB
Threads:	12
`
	name, rss, threads := parseProcStatus(content)
	if name != "harnez" {
		t.Errorf("parseProcStatus name = %q, want %q", name, "harnez")
	}
	if rss != 15984*1024 {
		t.Errorf("parseProcStatus rss = %d, want %d", rss, 15984*1024)
	}
	if threads != 12 {
		t.Errorf("parseProcStatus threads = %d, want 12", threads)
	}
}

func TestParseSystemdProperties(t *testing.T) {
	content := `ActiveState=active
SubState=running
MainPID=714116
MemoryCurrent=7864320
CPUUsageNSec=7935075000
`
	props := parseSystemdProperties(content)
	if props["ActiveState"] != "active" {
		t.Errorf("ActiveState = %q, want active", props["ActiveState"])
	}
	if props["MainPID"] != "714116" {
		t.Errorf("MainPID = %q, want 714116", props["MainPID"])
	}
}

func TestPrintVoiceResourceReport(t *testing.T) {
	report := VoiceResourceReport{
		Mode:          ModeEager,
		RecordStatus:  "idle",
		ActiveService: EagerService,
		ServiceStatus: "active (running)",
		ServicePID:    714116,
		ServiceMemory: 8 * 1024 * 1024,
		ServiceCPU:    5 * time.Second,
		GPUAccel:      "AMD Radeon Graphics (Vulkan 1.4 GPU)",
		ActiveModel:   "small.en",
		Processes: []ProcessResource{
			{PID: 714116, Name: "harnez", Cmdline: "harnez tools voice-input eager --daemon", RSSBytes: 8 * 1024 * 1024, Threads: 12},
		},
		ZombieWarnings: nil,
	}

	var buf bytes.Buffer
	PrintVoiceResourceReport(&buf, report)
	out := buf.String()

	if !strings.Contains(out, "Active Mode:") || !strings.Contains(out, "eager") {
		t.Errorf("Report missing active mode: %s", out)
	}
	if !strings.Contains(out, "0 orphan/zombie processes (clean)") {
		t.Errorf("Report missing health status: %s", out)
	}
}
