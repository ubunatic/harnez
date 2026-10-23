package usage

import (
	"os"
	"reflect"
	"testing"
)

func TestRemoteLoadCacheRoundTripByHost(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	want := &LoadSnapshot{CPU: CPULoad{NumCPU: 12, CPUPercent: 31, CPUPercentOk: true}}
	if err := WriteCachedRemoteLoad("host-a", want); err != nil {
		t.Fatal(err)
	}
	if got := ReadCachedRemoteLoad("host-a"); !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCachedRemoteLoad(host-a) = %#v, want %#v", got, want)
	}
	if got := ReadCachedRemoteLoad("host-b"); got != nil {
		t.Fatalf("ReadCachedRemoteLoad(host-b) = %#v, want nil", got)
	}
	entries, err := os.ReadDir(StateDir(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache files = %d, want 1", len(entries))
	}
}
