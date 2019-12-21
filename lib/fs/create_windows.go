// +build windows

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
	procLockFileEx = modkernel32.NewProc("LockFileEx")
	// https://docs.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createeventexw
	procCreateEventW = modkernel32.NewProc("CreateEventW")
	// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdiskfreespaceexw
	getFreeSpace = modkernel32.NewProc("GetDiskFreeSpaceExW")
)

const (
	// The function requests an exclusive lock. Otherwise, it requests a shared lock.
	lockfileExclusiveLock = 0x00000002

	fileFlagNormal = 0x00000080
)

func lock(filename string) (err error) {
	name, err := syscall.UTF16PtrFromString(filename)
	if err != nil {
		return err
	}

	// Open for asynchronous I/O so that we can timeout waiting for the lock.
	// Also open shared so that other processes can open the file (but will
	// still need to lock it).
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_FLAG_OVERLAPPED|fileFlagNormal,
		0)
	if err != nil {
		return err
	}
	// close handle if error returns
	defer func() {
		if err != nil {
			if err := syscall.Close(handle); err != nil {
				logger.Errorf("Falied to close handle: %q", err)
			}
		}
	}()

	ol, err := newOverlapped()
	defer func() { _ = syscall.CloseHandle(ol.HEvent) }()
	err = lockFileEx(handle, ol)
	if err == nil {
		return nil
	}

	s, err := syscall.WaitForSingleObject(ol.HEvent, uint32(syscall.INFINITE))

	switch s {
	case syscall.WAIT_OBJECT_0:
		// success!
		return nil
	default:
		return err
	}
}

func newOverlapped() (ol *syscall.Overlapped, err error) {
	r0, _, errno := syscall.Syscall6(procCreateEventW.Addr(), 4,
		// *syscall.SecurityAttributes
		uintptr(unsafe.Pointer(nil)),
		// manual reset (bool)
		uintptr(1),
		// initial state (bool)
		uintptr(0),
		// name (string: *uint16)
		uintptr(unsafe.Pointer(nil)),
		// blank params
		0, 0)
	overlappedHandle := syscall.Handle(r0)
	if overlappedHandle == syscall.InvalidHandle {
		if errno != 0 {
			err = errno
		} else {
			err = syscall.EINVAL
		}
		return nil, err
	}
	return &syscall.Overlapped{HEvent: overlappedHandle}, nil
}

func lockFileEx(h syscall.Handle, ol *syscall.Overlapped) (err error) {
	r1, _, e1 := syscall.Syscall6(procLockFileEx.Addr(), 6,
		// hFile: handle to the file
		uintptr(h),
		//dwFlags: This parameter may be 1 2 3
		uintptr(lockfileExclusiveLock),
		// dwReserved: Reserved parameter; must be set to zero.
		uintptr(0),
		// nNumberOfBytesToLockHigh: The high-order 32 bits of the length of the byte range to lock.
		uintptr(1),
		// nNumberOfBytesToLockLow: The low-order 32 bits of the length of the byte range to lock.
		uintptr(0),
		// lpOverlapped: A pointer to an OVERLAPPED structure that the function uses with the locking request.
		uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
func CreateFlockFile(dir string) (*os.File, error) {
	flockName := filepath.Join(dir, "flock.lock")
	logger.Infof("file lock on %q", flockName)

	// winlock := fslock.New(flockName)
	if err := lock(flockName); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %q", flockName, err)
	}

	flockF, err := os.Open(flockName)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %q", flockName, err)
	}
	return flockF, nil
}

// MustGetFreeSpace returns free space for the given directory path.
func MustGetFreeSpace(path string) uint64 {
	lpFreeBytesAvailable := uint64(0)
	lpTotalNumberOfBytes := uint64(0)
	lpTotalNumberOfFreeBytes := uint64(0)
	_, _, err := syscall.Syscall6(getFreeSpace.Addr(), 4,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(filepath.VolumeName(path)))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfFreeBytes)),
		0, 0)

	if err != 0 {
		fmt.Printf("This is error: %q\n", err)
		logger.Panicf("FATAL: cannot  determine free disk space on %q: %q", path, err)
	}
	return lpFreeBytesAvailable
}
