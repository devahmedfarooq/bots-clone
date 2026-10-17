package diagnostics

import (
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")

func systemMemoryAndDisk() (any, any) {
	m := struct {
		Length, Load                                                                                         uint32
		TotalPhys, AvailPhys, TotalPageFile, AvailPageFile, TotalVirtual, AvailVirtual, AvailExtendedVirtual uint64
	}{}
	m.Length = uint32(unsafe.Sizeof(m))
	ok, _, _ := kernel.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&m)))
	memory := map[string]uint64{}
	if ok != 0 {
		memory["total_bytes"] = m.TotalPhys
		memory["available_bytes"] = m.AvailPhys
	}
	disks := map[string]uint64{}
	var free, total, allFree uint64
	ok, _, _ = kernel.NewProc("GetDiskFreeSpaceExW").Call(0, uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&allFree)))
	if ok != 0 {
		disks["workspace_available_bytes"] = free
		disks["workspace_total_bytes"] = total
	}
	return memory, disks
}
