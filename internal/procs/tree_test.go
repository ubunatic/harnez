package procs

import (
	"context"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

type treeReader struct{ processes []ProcInfo }

func (r *treeReader) List() ([]ProcInfo, error) { return r.processes, nil }

func TestStopTreeSignalEscalationAndConfirmation(t *testing.T) {
	for _, outcome := range []string{"term", "kill", "survives"} {
		t.Run(outcome, func(t *testing.T) {
			reader := &treeReader{[]ProcInfo{{PID: 987654, PGID: 987654, Starttime: 42, State: "S", UIDKnown: true, UID: uint32(os.Getuid())}}}
			var signals []syscall.Signal
			live, err := StopTree(context.Background(), TreeOptions{Roots: []TreeRoot{{PID: 987654, Starttime: 42, Group: true}}, Reader: reader, Grace: time.Millisecond, KillWait: time.Millisecond, Poll: time.Millisecond, Signal: func(pid int, signal syscall.Signal) error {
				if pid != 987654 {
					t.Fatalf("unexpected signal target %d", pid)
				}
				signals = append(signals, signal)
				if outcome == "term" || (outcome == "kill" && signal == syscall.SIGKILL) {
					reader.processes = nil
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			want := []syscall.Signal{syscall.SIGTERM}
			if outcome != "term" {
				want = append(want, syscall.SIGKILL)
			}
			if !reflect.DeepEqual(signals, want) {
				t.Fatalf("signals = %v, want %v", signals, want)
			}
			if (len(live) > 0) != (outcome == "survives") {
				t.Fatalf("untruthful exit confirmation: %+v", live)
			}
		})
	}
}

func TestTreeMembersTracksReparentedProcessesAndRejectsReuse(t *testing.T) {
	proc := func(pid, ppid, pgid int, start uint64, state string) ProcInfo {
		return ProcInfo{PID: pid, PPID: ppid, PGID: pgid, Starttime: start, State: state, UIDKnown: true, UID: uint32(os.Getuid())}
	}
	reader := &treeReader{[]ProcInfo{proc(100, 1, 100, 5, "S"), proc(101, 100, 101, 6, "S"), proc(102, 101, 101, 7, "S")}}
	known := make(map[int]ProcInfo)
	roots := []TreeRoot{{PID: 100, Starttime: 5, Group: true}}
	live, err := TreeMembers(roots, known, reader)
	if err != nil || len(live) != 3 {
		t.Fatalf("initial tree = %+v, %v", live, err)
	}
	reader.processes = []ProcInfo{proc(101, 1, 101, 6, "S"), proc(102, 101, 101, 7, "Z")}
	live, err = TreeMembers(roots, known, reader)
	if err != nil || len(live) != 1 || live[0].PID != 101 {
		t.Fatalf("reparented tree = %+v, %v", live, err)
	}
	reader.processes = []ProcInfo{proc(100, 1, 100, 50, "S"), proc(110, 100, 100, 51, "S")}
	live, err = TreeMembers(roots, known, reader)
	if err != nil || len(live) != 0 {
		t.Fatalf("recycled PID selected unrelated tree = %+v, %v", live, err)
	}
}

func TestTreeMembersFindsGroupAfterWrapperAndLeaderExit(t *testing.T) {
	reader := &treeReader{[]ProcInfo{{PID: 201, PPID: 1, PGID: 200, Starttime: 6, State: "S", UIDKnown: true, UID: uint32(os.Getuid())}}}
	live, err := TreeMembers([]TreeRoot{{PID: 200, Starttime: 5, Group: true}}, make(map[int]ProcInfo), reader)
	if err != nil || len(live) != 1 {
		t.Fatalf("orphaned provider group = %+v, %v", live, err)
	}
}
