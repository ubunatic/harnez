// Package modelcatalog loads harnez's merged model catalog: the embedded spec
// defaults, machine state, and the user's policy. Entry point: [Load].
//
// Stable API: exported identifiers in this package (types, fields, methods,
// functions) are a public contract for external consumers such as taia. An
// exported name is never removed, renamed, or changed incompatibly in place.
// The change goes through a deprecation step first: the old form gets a
// "Deprecated:" comment that names its replacement, and it stays available
// until a later, documented removal.
package modelcatalog
