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

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{2*time.Minute + 15*time.Second, "2m 15s"},
		{1*time.Hour + 5*time.Minute + 3*time.Second, "1h 5m 3s"},
	}
	for _, tc := range cases {
		if got := FormatDuration(tc.d); got != tc.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestRenderSparkline(t *testing.T) {
	values := []float64{0, 2.5, 5.0, 7.5, 10.0}
	spark := RenderSparkline(values, 10.0)
	if len(spark) == 0 {
		t.Fatalf("expected non-empty sparkline")
	}
	if !strings.ContainsRune(spark, ' ') || !strings.ContainsRune(spark, '█') {
		t.Errorf("expected sparkline to scale from min to max: %q", spark)
	}
}

func TestRenderSpeedGauge(t *testing.T) {
	gaugeFast := RenderSpeedGauge(0.08)
	if !strings.Contains(gaugeFast, "█") {
		t.Errorf("expected filled speed gauge for fast RTF: %q", gaugeFast)
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

func TestParseSections(t *testing.T) {
	all := ParseSections("all")
	if !all.Speed || !all.Hardware || !all.Transcript || !all.Daemons {
		t.Errorf("expected all sections enabled, got %+v", all)
	}

	custom := ParseSections("s,t")
	if !custom.Speed || custom.Hardware || !custom.Transcript || custom.Daemons {
		t.Errorf("expected only s and t enabled, got %+v", custom)
	}

	named := ParseSections("hardware,daemons")
	if named.Speed || !named.Hardware || named.Transcript || !named.Daemons {
		t.Errorf("expected only hardware and daemons enabled, got %+v", named)
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
		ServiceUptime: 100 * time.Second,
		AvgCPULoad:    5.0,
		LiveCPULoad:   1.2,
		CPUSparkline:  " ▂▃▅",
		GPUAccel:      "AMD Radeon Vulkan 1.4",
		ActiveModel:   "small.en",
		Processes: []ProcessResource{
			{PID: 714116, Name: "harnez", Cmdline: "harnez tools voice-input eager --daemon", RSSBytes: 8 * 1024 * 1024, Threads: 12},
		},
		EagerMetrics: &EagerMetrics{
			TotalChunks:         2,
			TotalAudioSecs:      5.0,
			TotalTranscribeSecs: 0.5,
			AvgRTF:              0.10,
			LastUtterance: &UtteranceStat{
				Index:          2,
				AudioSecs:      2.5,
				TranscribeSecs: 0.25,
				RTF:            0.10,
				Text:           "Test speech",
				Timestamp:      time.Now(),
			},
			Recent: []UtteranceStat{
				{Index: 2, AudioSecs: 2.5, TranscribeSecs: 0.25, RTF: 0.10, Text: "Test speech", Timestamp: time.Now()},
			},
		},
		ZombieWarnings: nil,
	}

	var buf bytes.Buffer
	PrintVoiceResourceReport(&buf, report, DefaultResourceSections())
	out := StripANSI(buf.String())

	if !strings.Contains(out, "status:") || !strings.Contains(out, "eager") {
		t.Errorf("Report missing active mode: %s", out)
	}
	if !strings.Contains(out, "[s]peed") || !strings.Contains(out, "[h]ardware") {
		t.Errorf("Report missing footer buttons: %s", out)
	}
	if !strings.Contains(out, "speed:") {
		t.Errorf("Report missing speed line: %s", out)
	}
}
