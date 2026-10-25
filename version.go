package harnez

var Version = "0.2.1"

// CommitCount is set by release and development build commands. An empty value
// identifies a build without comparable commit metadata.
var CommitCount = ""

// BuildVersion reports the release version and, when available, its commit count.
func BuildVersion() string {
	if CommitCount == "" {
		return Version
	}
	return Version + "+" + CommitCount
}
