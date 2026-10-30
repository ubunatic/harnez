package claude

import "io"

// ApplyOptions selects components and optional installations for a global apply.
// A nil Selection means all components; an omitted DocVariant uses full sources.
type ApplyOptions struct {
	Selection      Set
	Docs           []string
	ForceDocs      bool
	InstallSystemd bool
	InstallShell   bool
	DocVariant     string
}

// InitOptions selects project initialization behavior. IssuesGit is tri-state:
// nil preserves the recorded setting, true enables it, and false disables it;
// integration is installed only when the repo selects the Harnez tracker.
// An omitted Variant preserves each installed doc's variant.
type InitOptions struct {
	Docs     []string
	RepoMode string
	// ProtectedMain is set only when --protected-main was passed; nil keeps
	// the value recorded in repo.yaml.
	ProtectedMain *bool
	AssumeYes     bool
	WithSummary   bool
	Update        bool
	Replace       bool
	IssuesGit     *bool
	GoWork        bool
	Force         bool
	Variant       string
	Quota1        bool
	// WizardInput opts into interactive configuration; nil keeps the legacy path.
	// AssumeYes always bypasses it. WizardOutput defaults to stdout when omitted.
	WizardInput  io.Reader
	WizardOutput io.Writer
}
