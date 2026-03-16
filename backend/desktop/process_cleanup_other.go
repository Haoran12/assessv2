//go:build !windows

package main

func cleanupDescendantProcesses(rootPID uint32) error {
	return nil
}
