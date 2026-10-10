package usage

import (
	"os"
	"path/filepath"
	"strings"
)

// GPUClass is the internal kind of a GPU (issue 765). It only decides which
// memory rows the Load box draws; it is never printed. The zero value is
// GPUClassDGPU, so a snapshot from an older remote binary without the field
// renders as a discrete card.
type GPUClass int

const (
	GPUClassDGPU GPUClass = iota
	GPUClassIGPU
	GPUClassEGPU
)

// apuVRAMCarveoutMaxMiB is the largest VRAM reservation still treated as an
// APU's integrated GPU. An APU reserves a small slice of system RAM as VRAM
// and uses GTT for the rest; a discrete card reports several GiB of VRAM.
//
// UNVERIFIED ON HARDWARE: this is a heuristic, not checked against a real
// APU yet. The host checks it on the remote x600 (issue 765).
// TODO(765 M2): move to spec/usage.yaml with a schema entry; Go must not
// keep this as a default.
const apuVRAMCarveoutMaxMiB = 2048

// classifyAMDGPU maps the sysfs facts of one AMD GPU to its class. An
// external (removable PCI path) GPU is an eGPU; otherwise a small VRAM
// carve-out marks an iGPU, and everything else is a dGPU.
func classifyAMDGPU(external, haveVRAM bool, vramTotalMiB float64) GPUClass {
	if external {
		return GPUClassEGPU
	}
	if haveVRAM && vramTotalMiB <= apuVRAMCarveoutMaxMiB {
		return GPUClassIGPU
	}
	return GPUClassDGPU
}

// isExternalGPU reports whether the PCI function at deviceDir, or a PCI
// bridge above it, is marked removable by the kernel (an external or hotplug
// port, such as a Thunderbolt/USB4 enclosure). The walk climbs from the
// device to its root complex (pciNNNN:NN) and stops there.
//
// UNVERIFIED ON HARDWARE: the eGPU on the remote x600 must be checked live;
// no local eGPU was available to calibrate against (issue 765).
func isExternalGPU(deviceDir string) bool {
	dir, err := filepath.EvalSymlinks(deviceDir)
	if err != nil {
		return false
	}
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "removable")); err == nil && strings.TrimSpace(string(data)) == "removable" {
			return true
		}
		if strings.HasPrefix(filepath.Base(dir), "pci") {
			return false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
