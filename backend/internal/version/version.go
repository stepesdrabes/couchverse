// Package version identifies the running server to clients.
package version

// Version is the repository version the binary was built from, stamped at build
// time with -ldflags "-X couchverse/internal/version.Version=...".
var Version = "dev"

// APILevel is bumped whenever clients need new server behaviour. Clients declare
// the minimum level they support and refuse older servers.
const APILevel = 2
