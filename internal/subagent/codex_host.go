package subagent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// IsInteractiveCodexThread verifies the exact thread's metadata. Unknown,
// unreadable and exec/subagent sources never grant host privileges.
func IsInteractiveCodexThread(threadID string) bool {
	if threadID == "" || strings.ContainsAny(threadID, `/\*?[]`) {
		return false
	}
	spec, err := agentSpecOnce()
	if err != nil {
		return false
	}
	discovery := spec.ExternalSessions["codex"]
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	root := strings.ReplaceAll(discovery.Root, "{home}", home)
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		root = filepath.Join(codexHome, "sessions")
	}
	paths, err := filepath.Glob(filepath.Join(root, discovery.Pattern))
	if err != nil {
		return false
	}
	for _, path := range paths {
		if !strings.HasSuffix(path, "-"+threadID+".jsonl") {
			continue
		}
		sess, err := readExternalSession("codex", discovery, path)
		if err == nil && sess.ProviderID() == threadID && slices.Contains(discovery.InteractiveSources, sess.Source) {
			return true
		}
	}
	return false
}
