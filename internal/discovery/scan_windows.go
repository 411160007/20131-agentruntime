//go:build windows

package discovery

import (
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Snapshot builds the process table with kernel32/ntdll primitives:
//
//   - CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS) → pid, ppid, exe name
//   - QueryFullProcessImageNameW → full executable path
//   - NtQueryInformationProcess(ProcessBasicInformation) + ReadProcessMemory
//     → PEB.ProcessParameters.CommandLine (the classic no-cmdline problem
//     on Windows: there is no /proc, the command line lives in the PEB).
//
// Honest-scope notes (deviations recorded, not hidden):
//   - Real-time ETW user-space process providers are the planned Phase 1
//     upgrade (sub-second event stream); v0 polls a Toolhelp32 snapshot,
//     which is the same user-space view, polling instead of streaming.
//   - Rows for processes protected from PROCESS_QUERY_LIMITED_INFORMATION
//     (some anti-malware/system services) keep only name+pid+ppid; their
//     count lands in ScanStats via empty Cmdline (coverage honesty).
var (
	modkernel32                = syscall.NewLazyDLL("kernel32.dll")
	modntdll                   = syscall.NewLazyDLL("ntdll.dll")
	procCreateToolhelp32Snap   = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW        = modkernel32.NewProc("Process32FirstW")
	procProcess32NextW         = modkernel32.NewProc("Process32NextW")
	procQueryFullImageName     = modkernel32.NewProc("QueryFullProcessImageNameW")
	procReadProcessMemory      = modkernel32.NewProc("ReadProcessMemory")
	procNtQueryInformationProc = modntdll.NewProc("NtQueryInformationProcess")
)

const (
	th32csSnapProcess          = 0x00000002
	processQueryLimitedInfo    = 0x1000
	processBasicInformationCls = 0
	pebOffsetProcessParams     = 0x20 // x64 PEB.ProcessParameters
	paramsOffsetCommandLine    = 0x70 // x64 RTL_USER_PROCESS_PARAMETERS.CommandLine
	invalidHandle              = ^uintptr(0)
	errNoMoreFiles             = syscall.Errno(18) // ERROR_NO_MORE_FILES
)

type processEntry32W struct {
	Size          uint32
	Usage         uint32
	ProcessID     uint32
	DefaultHeapID uintptr
	ModuleID      uint32
	Threads       uint32
	ParentProcess uint32
	PriorityClass int32
	Flags         uint32
	ExeFile       [260]uint16
}

// processBasicInformation mirrors the x64 layout (natural alignment).
type processBasicInformation struct {
	ExitStatus                   int32
	_                            uint32
	PebBaseAddress               uintptr
	AffinityMask                 uintptr
	BasePriority                 int32
	_                            uint32
	UniqueProcessID              uintptr
	InheritedFromUniqueProcessID uintptr
}

func Snapshot() ([]ProcInfo, error) {
	snap, _, err := procCreateToolhelp32Snap.Call(th32csSnapProcess, 0)
	if snap == invalidHandle {
		return nil, fmt.Errorf("discovery: CreateToolhelp32Snapshot failed: %v", err)
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	var entries []ProcInfo
	var e processEntry32W
	e.Size = uint32(unsafe.Sizeof(e))
	for first := true; ; first = false {
		var r uintptr
		if first {
			r, _, _ = procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
		} else {
			r, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
		}
		if r == 0 {
			break
		}
		p := ProcInfo{
			PID:  int(e.ProcessID),
			PPID: int(e.ParentProcess),
			Name: syscall.UTF16ToString(e.ExeFile[:]),
		}
		if ph, _, _ := procOpenProcessCall.Call(processQueryLimitedInfo, 0, uintptr(e.ProcessID)); ph != 0 {
			p.Exe = queryFullImageName(syscall.Handle(ph))
			p.Cmdline = readPebCommandLine(syscall.Handle(ph), e.ProcessID)
			syscall.CloseHandle(syscall.Handle(ph))
		}
		entries = append(entries, p)
	}
	if len(entries) == 0 {
		return nil, errors.New("discovery: toolhelp snapshot returned no processes")
	}
	return entries, nil
}

// hiddenFromUs reports whether the pid still exists but cannot be opened
// for the limited query rights (protected process) — coverage honesty.
func hiddenFromUs(pid int) bool {
	ph, _, _ := procOpenProcessCall.Call(processQueryLimitedInfo, 0, uintptr(pid))
	if ph == 0 {
		return true
	}
	syscall.CloseHandle(syscall.Handle(ph))
	return false
}

var procOpenProcessCall = modkernel32.NewProc("OpenProcess")

func queryFullImageName(h syscall.Handle) string {
	buf := make([]uint16, 512)
	size := uint32(len(buf))
	r, _, _ := procQueryFullImageName.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return ""
	}
	if int(size) > len(buf) {
		size = uint32(len(buf))
	}
	return string(utf16.Decode(buf[:size]))
}

func readPebCommandLine(h syscall.Handle, pid uint32) string {
	var pbi processBasicInformation
	var retLen uint32
	r, _, _ := procNtQueryInformationProc.Call(
		uintptr(h), processBasicInformationCls,
		uintptr(unsafe.Pointer(&pbi)), unsafe.Sizeof(pbi),
		uintptr(unsafe.Pointer(&retLen)))
	if r != 0 || pbi.PebBaseAddress == 0 {
		return ""
	}
	var paramsAddr uintptr
	if !rpm(h, pbi.PebBaseAddress+pebOffsetProcessParams, unsafe.Pointer(&paramsAddr), unsafe.Sizeof(paramsAddr)) || paramsAddr == 0 {
		return ""
	}
	// UNICODE_STRING (x64): Length uint16, MaximumLength uint16, pad 4, Buffer ptr.
	var us [16]byte
	if !rpm(h, paramsAddr+paramsOffsetCommandLine, unsafe.Pointer(&us[0]), 16) {
		return ""
	}
	strLen := binary.LittleEndian.Uint16(us[0:2])
	bufPtr := *(*uintptr)(unsafe.Pointer(&us[8:16][0]))
	if strLen == 0 || bufPtr == 0 || strLen > 32768 {
		return ""
	}
	utf16Buf := make([]uint16, strLen/2)
	if !rpm(h, bufPtr, unsafe.Pointer(&utf16Buf[0]), uintptr(strLen)) {
		return ""
	}
	return string(utf16.Decode(utf16Buf))
}

func rpm(h syscall.Handle, addr uintptr, dst unsafe.Pointer, size uintptr) bool {
	var done uintptr
	r, _, _ := procReadProcessMemory.Call(uintptr(h), addr, uintptr(dst), size, uintptr(unsafe.Pointer(&done)))
	return r != 0 && done == size
}
