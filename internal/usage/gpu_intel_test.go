package usage

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// intelFixture is a fake /sys/class/drm tree with one i915 card per PCI
// address, driven by a fake clock so no test waits on real time.
type intelFixture struct {
	t     *testing.T
	drm   string
	pci   string
	now   time.Time
	cards map[string]string // card name -> rc6 path
}

func newIntelFixture(t *testing.T) *intelFixture {
	t.Helper()
	root := t.TempDir()
	f := &intelFixture{t: t, drm: filepath.Join(root, "drm"), pci: filepath.Join(root, "pci"), now: time.Unix(1_000_000, 0), cards: map[string]string{}}
	for _, d := range []string{f.drm, f.pci, filepath.Join(root, "drivers", "i915"), filepath.Join(root, "drivers", "xe")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	intelDRMRoot = f.drm
	intelClock = func() time.Time { return f.now }
	intelSleep = func(d time.Duration) { f.now = f.now.Add(d) }
	intelLspci = func(string) (string, error) { return "Alder Lake-S GT1 [UHD Graphics 730]", nil }
	intelMu.Lock()
	intelLast = map[string]intelRC6Sample{}
	intelNameByPCI = map[string]string{}
	intelMu.Unlock()
	t.Cleanup(func() {
		intelDRMRoot = "/sys/class/drm"
		intelClock = time.Now
		intelSleep = time.Sleep
		intelLspci = readIntelNameFromLspci
	})
	return f
}

// addCard creates cardName whose PCI function has vendor, class and driver,
// and an RC6 counter at rc6 ms (skipped when rc6 is negative).
func (f *intelFixture) addCard(cardName, pciAddr, vendor, class, driver string, rc6 int) {
	f.t.Helper()
	root := filepath.Dir(f.drm)
	dev := filepath.Join(f.pci, pciAddr)
	if err := os.MkdirAll(dev, 0o755); err != nil {
		f.t.Fatal(err)
	}
	write(f.t, filepath.Join(dev, "vendor"), vendor+"\n")
	write(f.t, filepath.Join(dev, "class"), class+"\n")
	if driver != "" {
		if err := os.Symlink(filepath.Join(root, "drivers", driver), filepath.Join(dev, "driver")); err != nil {
			f.t.Fatal(err)
		}
	}
	card := filepath.Join(f.drm, cardName)
	if err := os.MkdirAll(card, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Symlink(dev, filepath.Join(card, "device")); err != nil {
		f.t.Fatal(err)
	}
	f.cards[cardName] = filepath.Join(card, "gt", "gt0", "rc6_residency_ms")
	if rc6 >= 0 {
		if err := os.MkdirAll(filepath.Dir(f.cards[cardName]), 0o755); err != nil {
			f.t.Fatal(err)
		}
		f.setRC6(cardName, rc6)
	}
}

func (f *intelFixture) setRC6(cardName string, ms int) {
	f.t.Helper()
	write(f.t, f.cards[cardName], strconv.Itoa(ms)+"\n")
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const intelIGPUClass = "0x030000"

func TestIntelBusyFromRC6Delta(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "i915", 1000)
	// The first sight seeds over one window: 250 ms of RC6 in a 500 ms window is 50% busy.
	slept := 0
	intelSleep = func(d time.Duration) {
		slept++
		f.now = f.now.Add(d)
		f.setRC6("card1", 1250)
	}
	gpus := currentIntelGPUs()
	if slept != 1 || len(gpus) != 1 || gpus[0].UtilPercent != 50 {
		t.Fatalf("first call: slept %d times, GPUs %+v; want one seed sleep and 50%% busy", slept, gpus)
	}
	// Next redraw: 1000 ms of wall time with 250 ms in RC6, 75% busy.
	f.now = f.now.Add(1000 * time.Millisecond)
	f.setRC6("card1", 1500)
	gpus = currentIntelGPUs()
	if slept != 1 || len(gpus) != 1 || gpus[0].UtilPercent != 75 {
		t.Fatalf("second call: slept %d times, GPUs %+v; want no new sleep and 75%% busy", slept, gpus)
	}
	g := gpus[0]
	if g.Name != "UHD 730" || g.PCI != "0000:00:02.0" || g.Class != GPUClassIGPU {
		t.Errorf("GPU = {Name:%q PCI:%q Class:%v}, want UHD 730 at 0000:00:02.0 as iGPU", g.Name, g.PCI, g.Class)
	}
	if g.UtilPercent != 75 {
		t.Errorf("UtilPercent = %v, want 75", g.UtilPercent)
	}
	if g.HaveMem || g.HaveVRAM || g.HaveGTT {
		t.Errorf("iGPU must not claim VRAM/GTT memory rows, got %+v", g)
	}
}

func TestIntelSeedSleepsOneWindowOnFirstSight(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "i915", 5000)
	var slept []time.Duration
	intelSleep = func(d time.Duration) {
		slept = append(slept, d)
		f.now = f.now.Add(d)
		f.setRC6("card1", 5000+200) // 200 ms of RC6 across the 500 ms window: 60% busy
	}
	gpus := currentIntelGPUs()
	if len(slept) != 1 || slept[0] != intelSeedWindow {
		t.Fatalf("seed slept %v, want one sleep of %v", slept, intelSeedWindow)
	}
	if len(gpus) != 1 || gpus[0].UtilPercent != 60 {
		t.Fatalf("seeded GPU = %+v, want one GPU at 60%%", gpus)
	}
	if len(gpus[0].UtilHistory) == 0 {
		t.Error("seeded GPU has no utilisation history")
	}
}

func TestIntelMissingBusySourceLeavesGPUOut(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "i915", -1) // no gt/gt0 counter
	if got := currentIntelGPUs(); len(got) != 0 {
		t.Fatalf("GPU without an RC6 counter must be left out, got %+v", got)
	}
}

func TestIntelSkipsOtherVendorsDriversAndConnectors(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "xe", 1000) // xe is not read
	f.addCard("card2", "0000:03:00.0", "0x1002", intelIGPUClass, "i915", 1000)    // AMD vendor
	f.addCard("card3", "0000:04:00.0", intelVendorID, "0x020000", "i915", 1000)   // network class, not display
	if err := os.MkdirAll(filepath.Join(f.drm, "card1-DP-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Second)
	if got := currentIntelGPUs(); len(got) != 0 {
		t.Fatalf("want no Intel GPUs, got %+v", got)
	}
}

func TestIntelCounterResetSkipsOneSample(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "i915", 1000)
	currentIntelGPUs() // seeds the baseline
	f.now = f.now.Add(time.Second)
	f.setRC6("card1", 100) // counter went backwards after a reset
	if got := currentIntelGPUs(); len(got) != 0 {
		t.Fatalf("backwards counter must be skipped, got %+v", got)
	}
	f.now = f.now.Add(time.Second)
	f.setRC6("card1", 600)
	got := currentIntelGPUs()
	if len(got) != 1 || got[0].UtilPercent != 50 {
		t.Fatalf("after the reset, want 50%% busy, got %+v", got)
	}
}

func TestIntelDiscreteCardIsDGPU(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "i915", 1000)
	f.addCard("card2", "0000:03:00.0", intelVendorID, intelIGPUClass, "i915", 1000)
	currentIntelGPUs()
	f.now = f.now.Add(time.Second)
	f.setRC6("card1", 1000)
	f.setRC6("card2", 1000)
	got := currentIntelGPUs()
	if len(got) != 2 {
		t.Fatalf("want 2 Intel GPUs, got %d", len(got))
	}
	classes := map[string]GPUClass{}
	for _, g := range got {
		classes[g.PCI] = g.Class
	}
	if classes["0000:00:02.0"] != GPUClassIGPU || classes["0000:03:00.0"] != GPUClassDGPU {
		t.Errorf("classes = %v, want iGPU at 00:02.0 and dGPU at 03:00.0", classes)
	}
}

func TestIntelNameFallsBackWithoutLspci(t *testing.T) {
	f := newIntelFixture(t)
	f.addCard("card1", "0000:00:02.0", intelVendorID, intelIGPUClass, "i915", 1000)
	intelLspci = func(string) (string, error) { return "", errors.New("no lspci") }
	currentIntelGPUs()
	f.now = f.now.Add(time.Second)
	f.setRC6("card1", 1000)
	got := currentIntelGPUs()
	if len(got) != 1 || got[0].Name != "Intel GPU" {
		t.Fatalf("want fallback name, got %+v", got)
	}
}

func TestIntelShortName(t *testing.T) {
	tests := map[string]string{
		"Alder Lake-S GT1 [UHD Graphics 730]": "UHD 730",
		"Meteor Lake-P [Arc Graphics]":        "Arc",
		"Raptor Lake-P [Iris Xe Graphics]":    "Iris Xe",
		"Device 9a49":                         "Device 9a49",
	}
	for in, want := range tests {
		if got := intelShortName(in); got != want {
			t.Errorf("intelShortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsCardNode(t *testing.T) {
	for name, want := range map[string]bool{"card0": true, "card12": true, "card1-DP-1": false, "card": false, "renderD128": false} {
		if got := isCardNode(name); got != want {
			t.Errorf("isCardNode(%q) = %v, want %v", name, got, want)
		}
	}
}
