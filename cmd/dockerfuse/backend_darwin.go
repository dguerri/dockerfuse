//go:build darwin

package main

import "flag"

var backend string

func init() {
	flag.StringVar(&backend, "backend", "macfuse", "FUSE backend: 'macfuse' (default) or 'fskit' (requires macFUSE 5.2+, macOS 15.4+)")
}

// goFuseBackend maps the user-facing --backend flag to the value expected
// by go-fuse's MountOptions.Backend field. "macfuse" (the legacy kext path)
// corresponds to the empty string in go-fuse; "fskit" selects the FSKit
// backend.
func goFuseBackend() string {
	if backend == "fskit" {
		return "fskit"
	}
	return ""
}
