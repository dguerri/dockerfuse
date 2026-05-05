//go:build !darwin

package main

// goFuseBackend returns the empty string on non-darwin platforms; go-fuse
// only supports MountOptions.Backend == "fskit" on darwin.
func goFuseBackend() string {
	return ""
}
