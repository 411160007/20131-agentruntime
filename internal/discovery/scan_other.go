//go:build !linux && !darwin && !windows

package discovery

import "fmt"

// Snapshot is only implemented for linux, darwin, and windows. This file
// keeps the package compiling on other GOOS values (vet, tooling).
func Snapshot() ([]ProcInfo, error) {
	return nil, fmt.Errorf("discovery: process snapshot unsupported on this platform")
}

func hiddenFromUs(pid int) bool { return false }
