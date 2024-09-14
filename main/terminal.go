//go:build !windows

package main

func enableVirtualTerminalProcessing() {
	// Not required for non-Windows systems
	return
}
