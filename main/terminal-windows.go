//go:build windows

package main

import (
	"os"
	"golang.org/x/sys/windows"
)

// enableVirtualTerminalProcessing enables virtual terminal processing for Windows consoles.
func enableVirtualTerminalProcessing() {
	hOut := windows.Handle(os.Stdout.Fd())
	var mode uint32
	windows.GetConsoleMode(hOut, &mode)
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	windows.SetConsoleMode(hOut, mode)
}
