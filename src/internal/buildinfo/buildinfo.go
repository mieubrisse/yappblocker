package buildinfo

// Version is set at build time via ldflags:
//
//	go build -ldflags "-X github.com/mieubrisse/yappblocker/internal/buildinfo.Version=1.2.3"
//
// When not set (e.g., plain `go build`), it defaults to "dev".
var Version = "dev"
