//go:build windows

package main

import "golang.org/x/sys/windows"

// saveConsoleCP returns the current console input and output code pages.
func saveConsoleCP() (in, out uint32) {
	in, _ = windows.GetConsoleCP()
	out, _ = windows.GetConsoleOutputCP()
	return
}

// restoreConsoleCP restores the console input and output code pages.
func restoreConsoleCP(in, out uint32) {
	_ = windows.SetConsoleCP(in)
	_ = windows.SetConsoleOutputCP(out)
}
