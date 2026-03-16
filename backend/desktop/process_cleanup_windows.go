//go:build windows

package main

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func cleanupDescendantProcesses(rootPID uint32) error {
	tree, err := processTree()
	if err != nil {
		return err
	}

	descendants := collectDescendants(tree, rootPID)
	if len(descendants) == 0 {
		return nil
	}

	var problems []string
	for _, pid := range descendants {
		if err := terminateProcess(pid); err != nil {
			problems = append(problems, fmt.Sprintf("pid %d: %v", pid, err))
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf(strings.Join(problems, "; "))
	}
	return nil
}

func processTree() (map[uint32][]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	childrenByParent := make(map[uint32][]uint32)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return childrenByParent, nil
		}
		return nil, fmt.Errorf("read first process: %w", err)
	}

	for {
		childrenByParent[entry.ParentProcessID] = append(childrenByParent[entry.ParentProcessID], entry.ProcessID)
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, fmt.Errorf("read next process: %w", err)
		}
	}

	return childrenByParent, nil
}

func collectDescendants(tree map[uint32][]uint32, rootPID uint32) []uint32 {
	var descendants []uint32
	queue := append([]uint32{}, tree[rootPID]...)

	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		descendants = append(descendants, pid)
		queue = append(queue, tree[pid]...)
	}

	return descendants
}

func terminateProcess(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return nil
		}
		return err
	}
	return nil
}
