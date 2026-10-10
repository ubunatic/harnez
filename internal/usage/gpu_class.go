package usage

import (
	"os"
	"path/filepath"
	"strings"
)

// GPUClass is the internal kind of a GPU (issue 765). It only decides which
// memory rows the Load box draws; it is never printed. The zero value is
// GPUClassUnknown, so a snapshot from an older remote binary without the
// field keeps the pre-765 rendering (vram/gtt row whenever GTT is present).
type GPUClass int

const (
	GPUClassUnknown GPUClass = iota
	GPUClassDGPU
	GPUClassIGPU
	GPUClassEGPU
)

// classifyAMDGPU maps the sysfs facts of one AMD GPU to its class. An
// external (removable PCI path) GPU is an eGPU; otherwise an APU's metrics
// layout marks an iGPU, and everything else is a dGPU.
func classifyAMDGPU(external, apu bool) GPUClass {
	if external {
		return GPUClassEGPU
	}
	if apu {
		return GPUClassIGPU
	}
	return GPUClassDGPU
}

// isAPUGPU reports whether the amdgpu driver publishes APU-style metrics for
// deviceDir. The kernel writes gpu_metrics with a 4-byte header
// (structure_size u16, format_revision u8, content_revision u8); APUs use
// format_revision 2 (gpu_metrics_v2_x) and discrete cards use format 1
// (gpu_metrics_v1_x). Checked on the local Cezanne APU (format 2, content 2).
//
// UNVERIFIED ON HARDWARE for discrete cards: this machine has no dGPU to
// confirm they report format 1. The host checks the remote x600 (issue 765).
func isAPUGPU(deviceDir string) bool {
	data, err := os.ReadFile(filepath.Join(deviceDir, "gpu_metrics"))
	if err != nil || len(data) < 4 {
		return false
	}
	return data[2] == 2
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
