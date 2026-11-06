// Package quota holds the JSON types of harnez's usage --json output: the
// usage summary, per-agent usage and quota windows, and the load snapshot
// that rides along with them. It imports only the standard library.
//
// Stable API: exported identifiers in this package (types, fields, methods,
// functions) are a public contract for external consumers such as taia. An
// exported name is never removed, renamed, or changed incompatibly in place.
// The change goes through a deprecation step first: the old form gets a
// "Deprecated:" comment that names its replacement, or says that none exists
// yet, and it stays available until a later, documented removal.
//
// Wire format: fields that carry no json tag (CPULoad, SystemMemory, GPU,
// GPUClass) are encoded under their Go field names. Those names are the JSON
// keys, so adding a tag, renaming a field, or changing a field's type is a
// wire change and goes through the same deprecation step. GPUClass encodes
// as its integer value.
//
// Fields tagged `json:"-"` (for example AgentUsage.QuotaFetchDurationMS) are
// not part of the wire format. They are internal and outside the stable
// promise: they may change or be removed without a deprecation step.
package quota
