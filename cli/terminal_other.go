//go:build !windows

package main

func saveConsoleCP() (in, out uint32) { return 0, 0 }
func restoreConsoleCP(in, out uint32)  {}
