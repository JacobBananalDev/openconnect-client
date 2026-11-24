//go:build linux && 386

package ipc

import (
	"runtime"
	"syscall"
	"unsafe"
)

func getPeerGroupsSockopt(fd uintptr, groups []uint32, length *uint32) syscall.Errno {
	const socketcallGetsockopt = 15
	args := [5]uintptr{fd, syscall.SOL_SOCKET, soPeerGroups, uintptr(unsafe.Pointer(&groups[0])), uintptr(unsafe.Pointer(length))}
	_, _, errno := syscall.Syscall(syscall.SYS_SOCKETCALL, socketcallGetsockopt, uintptr(unsafe.Pointer(&args[0])), 0)
	runtime.KeepAlive(groups)
	runtime.KeepAlive(length)
	return errno
}
