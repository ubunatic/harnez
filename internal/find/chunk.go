package find

type ChunkKind string

const (
	KindPackageDoc ChunkKind = "package_doc"
	KindType       ChunkKind = "type"
	KindFunc       ChunkKind = "func"
	KindDocSection ChunkKind = "doc_section"
)

type Chunk struct {
	FilePath   string    `json:"file_path"`
	Package    string    `json:"package,omitempty"`
	Kind       ChunkKind `json:"kind"`
	Identifier string    `json:"identifier"`
	LineStart  int       `json:"line_start"`
	LineEnd    int       `json:"line_end"`
	Summary    string    `json:"summary"`
	Imports    []string  `json:"imports,omitempty"`
}

type IndexPayload struct {
	Version   int     `json:"version"`
	Generated string  `json:"generated_at"`
	RepoRoot  string  `json:"repo_root"`
	Chunks    []Chunk `json:"chunks"`
}

type DiscoveryResult struct {
	Query        string   `json:"query"`
	Packages     []string `json:"packages"`
	Patterns     []string `json:"detected_patterns"`
	Declarations []Chunk  `json:"key_declarations"`
	Docs         []Chunk  `json:"relevant_docs"`
}
