//go:build server

package services

// Server mode (go build -tags server) runs the app as a plain HTTP server
// with no native window, so it can be driven from any browser. See
// app/AGENTS.md, "Browser mode".
const mode = "browser"
