//go:build linux && !386

package ipc

import (
	"syscall"
	"unsafe"
)

func getPeerGroupsSockopt(fd uintptr, groups []uint32, length *uint32) syscall.Errno {
	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, syscall.SOL_SOCKET, soPeerGroups,
		uintptr(unsafe.Pointer(&groups[0])), uintptr(unsafe.Pointer(length)), 0)
	return errno
}
