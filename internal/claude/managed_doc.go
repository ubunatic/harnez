package claude

import (
	"bytes"
	"fmt"
	"os"

	"ubunatic.com/harnez/internal/markdown"
)

func hasDocStopMarker(data []byte) bool {
	for _, marker := range []string{markdown.DocStopMarker, markdown.DocEndMarker, markdown.LegacyDocStopMarker, markdown.LegacyDocEndMarker} {
		if bytes.Contains(data, []byte(marker)) {
			return true
		}
	}
	return false
}

// prepareManagedDoc adds the ownership boundary to new and unchanged installed
// docs. A differing legacy doc without a boundary needs human review.
func obviousLocalAppend(existing, bundled []byte) bool {
	managed := bytes.TrimRight(bundled, "\r\n")
	if !bytes.HasPrefix(existing, managed) {
		return false
	}
	tail := bytes.TrimLeft(existing[len(managed):], "\r\n")
	return (bytes.HasPrefix(tail, []byte("## ")) || bytes.HasPrefix(tail, []byte("# "))) && bytes.Count(tail, []byte("\n")) >= 2
}

func prepareManagedDoc(dst string, bundled []byte) ([]byte, string, error) {
	existing, err := os.ReadFile(dst)
	if err != nil && !os.IsNotExist(err) {
		return nil, "", err
	}
	if err == nil {
		if hasDocStopMarker(existing) {
			return markdown.MergeManagedDoc(dst, bundled), "", nil
		}
		if obviousLocalAppend(existing, bundled) {
			return nil, "", fmt.Errorf("%s has an appended local section without a stop marker; preserved it. Move that section below <!-- harnez:stop --> or to a project-owned file, then rerun init", dst)
		}
		if !bytes.Equal(existing, bundled) {
			return withDocStopMarker(bundled), fmt.Sprintf("warning: replacing differing markerless doc %s; review the git diff for local edits", dst), nil
		}
	}
	return withDocStopMarker(bundled), "", nil
}

func withDocStopMarker(bundled []byte) []byte {
	if hasDocStopMarker(bundled) {
		return bundled
	}
	managed := bytes.TrimRight(bundled, "\r\n")
	return append(append([]byte{}, managed...), []byte("\n\n"+markdown.DocStopMarker+"\n")...)
}

func writeManagedDoc(dst string, bundled []byte) (applyResult, error) {
	merged, warning, err := prepareManagedDoc(dst, bundled)
	if err != nil {
		return applyResult{}, err
	}
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	return writeFileIfChanged(dst, merged)
}
