package main

import (
	"fmt"
	"os"
	"time"
)

// nonInteractiveExecArgs returns docker exec args to run claude inside an
// existing container without a TTY.
func nonInteractiveExecArgs(containerName string, claudeArgs []string) []string {
	args := []string{"exec", "-i", containerName, "claude"}
	return append(args, claudeArgs...)
}

// adaptRunArgsForNonInteractive returns a copy of runArgs with "-it" replaced
// by "-i". When niContainerName is non-empty the "--name <value>" pair is kept
// and the name value is replaced with niContainerName, allowing the container
// to be found by a concurrently-running agentjail process. When niContainerName
// is empty the "--name <value>" pair is dropped entirely (legacy behaviour).
func adaptRunArgsForNonInteractive(runArgs []string, niContainerName string) []string {
	out := make([]string, 0, len(runArgs))
	skipNext := false
	for _, arg := range runArgs {
		if skipNext {
			if niContainerName != "" {
				out = append(out, niContainerName)
			}
			// empty niContainerName: drop the name value (and the --name flag below)
			skipNext = false
			continue
		}
		if arg == "-it" {
			out = append(out, "-i")
			continue
		}
		if arg == "--name" {
			if niContainerName != "" {
				out = append(out, arg) // keep --name flag; value replaced on next iteration
			}
			// empty niContainerName: skip --name flag (and its value via skipNext)
			skipNext = true
			continue
		}
		out = append(out, arg)
	}
	return out
}

// tryNILock attempts to atomically create a lock file at path.
// Returns (file, true) if the lock was acquired, (nil, false) otherwise.
// A lock file older than 60 seconds is considered stale and will be removed.
func tryNILock(path string) (*os.File, bool) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err == nil {
		return f, true
	}
	if !os.IsExist(err) {
		return nil, false
	}
	// Lock exists — check if it is stale (owner process crashed).
	if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 60*time.Second {
		os.Remove(path)
		f2, err2 := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err2 == nil {
			return f2, true
		}
	}
	return nil, false
}

// releaseNILock closes and removes the lock file.
func releaseNILock(f *os.File) {
	path := f.Name()
	f.Close()
	os.Remove(path)
}

// niContainerName returns the non-interactive container name for a given prefix.
func niContainerNameForPrefix(prefix string) string {
	return fmt.Sprintf("agentjail-ni.%s", prefix)
}
