package agy

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/jsonc"
)

// TranslateClaudeRule maps one Claude permission rule to the agy rule with the
// same reach (issue 1002, canaries 2 and 3). agy matches command(<prefix>) on a
// word boundary and read_file/write_file(<dir>) on the whole directory. Rules
// agy cannot express (mid-pattern wildcards, MCP, WebFetch, other tools) return
// ok=false so the caller can report them instead of guessing.
func TranslateClaudeRule(rule, home string) (string, bool) {
	tool, arg, ok := splitRule(rule)
	if !ok {
		return "", false
	}
	switch tool {
	case "Bash":
		prefix := strings.TrimSuffix(strings.TrimSuffix(arg, " *"), ":*")
		if prefix == "" || strings.Contains(prefix, "*") {
			return "", false
		}
		return "command(" + prefix + ")", true
	case "Read", "Edit", "Write":
		path, ok := rulePath(arg, home)
		if !ok {
			return "", false
		}
		if tool == "Read" {
			return "read_file(" + path + ")", true
		}
		return "write_file(" + path + ")", true
	}
	return "", false
}

func splitRule(rule string) (tool, arg string, ok bool) {
	open := strings.IndexByte(rule, '(')
	if open <= 0 || !strings.HasSuffix(rule, ")") {
		return "", "", false
	}
	return rule[:open], rule[open+1 : len(rule)-1], true
}

// rulePath converts a Claude path pattern (~/x, //abs, dir/**) to an absolute path.
func rulePath(arg, home string) (string, bool) {
	path := strings.TrimSuffix(strings.TrimSuffix(arg, "/**"), "/*")
	switch {
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(home, path[2:])
	case strings.HasPrefix(path, "//"):
		path = path[1:]
	default:
		return "", false // project-relative patterns have no fixed agy path
	}
	if strings.ContainsAny(path, "*?[") {
		return "", false
	}
	return path, true
}

// expandRule expands a leading ~/ inside an agy rule argument.
func expandRule(rule, home string) string {
	if kind, arg, ok := splitRule(rule); ok && strings.HasPrefix(arg, "~/") {
		return kind + "(" + filepath.Join(home, arg[2:]) + ")"
	}
	return rule
}

// TranslateRules builds the agy allow/deny lists from Claude's rules plus the
// spec extras, and returns the Claude rules it could not map.
func TranslateRules(claudeAllow, claudeDeny []string, extra PermissionsSpec, home string) (allow, deny, unmapped []string) {
	add := func(dst []string, rule string) []string {
		if slices.Contains(dst, rule) {
			return dst
		}
		return append(dst, rule)
	}
	for _, r := range claudeAllow {
		if a, ok := TranslateClaudeRule(r, home); ok {
			allow = add(allow, a)
		} else {
			unmapped = append(unmapped, r)
		}
	}
	for _, r := range claudeDeny {
		if a, ok := TranslateClaudeRule(r, home); ok {
			deny = add(deny, a)
		} else {
			unmapped = append(unmapped, r)
		}
	}
	for _, r := range extra.Allow {
		allow = add(allow, expandRule(r, home))
	}
	for _, r := range extra.Deny {
		deny = add(deny, expandRule(r, home))
	}
	return allow, deny, unmapped
}

// PermissionRules translates Claude's rules with the embedded spec extras.
func PermissionRules(claudeAllow, claudeDeny []string) (allow, deny, unmapped []string) {
	home, _ := os.UserHomeDir()
	return TranslateRules(claudeAllow, claudeDeny, MustLoadSpec().Permissions, home)
}

func permissionsObject(doc map[string]any, path string) (map[string]any, error) {
	raw, exists := doc["permissions"]
	if !exists {
		return map[string]any{}, nil
	}
	perm, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("agy permissions: existing permissions in %s is not an object", path)
	}
	return perm, nil
}

func stringList(raw any) []string {
	items, _ := raw.([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func writeSettings(path string, doc map[string]any) (bool, error) {
	data := append(jsonc.MarshalPretty(doc), '\n')
	if err := jsonc.Validate(data); err != nil {
		return false, err
	}
	old, _ := os.ReadFile(path)
	if string(old) == string(data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, data, 0644)
}

// ApplyPermissions adds allow/deny rules to agy's settings.json, keeping the
// user's own rules and every other key. It never sets toolPermission.
func ApplyPermissions(path string, allow, deny []string) (bool, error) {
	doc := jsonc.Read(path)
	perm, err := permissionsObject(doc, path)
	if err != nil {
		return false, err
	}
	for key, rules := range map[string][]string{"allow": allow, "deny": deny} {
		list := stringList(perm[key])
		for _, r := range rules {
			if !slices.Contains(list, r) {
				list = append(list, r)
			}
		}
		if len(list) > 0 {
			perm[key] = list
		}
	}
	if len(perm) == 0 {
		return false, nil
	}
	doc["permissions"] = perm
	return writeSettings(path, doc)
}

// PermissionsStatus reports how many wanted rules are missing from settings.json.
func PermissionsStatus(path string, allow, deny []string) (missing int) {
	perm, _ := jsonc.Read(path)["permissions"].(map[string]any)
	for key, rules := range map[string][]string{"allow": allow, "deny": deny} {
		have := stringList(perm[key])
		for _, r := range rules {
			if !slices.Contains(have, r) {
				missing++
			}
		}
	}
	return missing
}

// RemovePermissions removes the given rules and leaves user-added rules alone.
func RemovePermissions(path string, allow, deny []string) (bool, error) {
	doc := jsonc.Read(path)
	perm, err := permissionsObject(doc, path)
	if err != nil || len(perm) == 0 {
		return false, err
	}
	for key, rules := range map[string][]string{"allow": allow, "deny": deny} {
		if _, ok := perm[key]; !ok {
			continue
		}
		list := slices.DeleteFunc(stringList(perm[key]), func(s string) bool { return slices.Contains(rules, s) })
		if len(list) == 0 {
			delete(perm, key)
		} else {
			perm[key] = list
		}
	}
	if len(perm) == 0 {
		delete(doc, "permissions")
	} else {
		doc["permissions"] = perm
	}
	return writeSettings(path, doc)
}
