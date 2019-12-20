// +build windows
package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procCreateEventW = modkernel32.NewProc("CreateEventW")
	getFreeSpace     = modkernel32.NewProc("GetDiskFreeSpaceExW")

	ErrTimeout = errors.New("Timeout")
)

const (
	lockfileExclusiveLock = 2
	fileFlagNormal        = 0x00000080
)

func Lock(filename string) (err error) {
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
	defer func() {
		if err != nil {
			syscall.Close(handle)
		}
	}()

	// creates a structure used to track asynchronous
	// I/O requests that have been issued
	event, err := createEvent(nil, true, false, nil)
	if err != nil {
		return err
	}
	ol := &syscall.Overlapped{HEvent: event}
	defer syscall.CloseHandle(event)
	err = lockFileEx(handle, lockfileExclusiveLock, 0, 1, 0, ol)
	if err == nil {
		return nil
	}

	// ERROR_IO_PENDING is expected when we're waiting on an asychronous event
	// to occur.
	if err != syscall.ERROR_IO_PENDING {
		return err
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

func lockFileEx(h syscall.Handle, flags, reserved, locklow, lockhigh uint32, ol *syscall.Overlapped) (err error) {
	r1, _, e1 := syscall.Syscall6(procLockFileEx.Addr(), 6, uintptr(h), uintptr(flags), uintptr(reserved), uintptr(locklow), uintptr(lockhigh), uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func createEvent(sa *syscall.SecurityAttributes, manualReset bool, initialState bool, name *uint16) (handle syscall.Handle, err error) {
	var _p0 uint32
	if manualReset {
		_p0 = 1
	}
	var _p1 uint32
	if initialState {
		_p1 = 1
	}

	r0, _, e1 := syscall.Syscall6(procCreateEventW.Addr(), 4, uintptr(unsafe.Pointer(sa)), uintptr(_p0), uintptr(_p1), uintptr(unsafe.Pointer(name)), 0, 0)
	handle = syscall.Handle(r0)
	if handle == syscall.InvalidHandle {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// func lock(filename string) error {
// 	err := os.Remove(filename)
// 	if err != nil && len(err.Error()) > 79 &&
// 		err.Error()[len(err.Error())-79:] == "The process cannot access the file because it is being used by another process." {
// 		return ErrFileIsBeingUsed
// 	}
// 	if err != nil && len(err.Error()) > 42 &&
// 		err.Error()[len(err.Error())-42:] != "The system cannot find the file specified." {
// 		return fmt.Errorf("Remove error: %v", err)
// 	}

// 	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
// 	if err != nil && len(err.Error()) > 79 &&
// 		err.Error()[len(err.Error())-79:] == "The process cannot access the file because it is being used by another process." {
// 		return ErrFileIsBeingUsed
// 	}
// 	if err != nil {
// 		return fmt.Errorf("OpenFile error: %v", err)
// 	}

// 	_, err = file.WriteString(strconv.FormatInt(int64(os.Getpid()), 10))
// 	if err != nil {
// 		return fmt.Errorf("WriteString error: %v", err)
// 	}

// 	return nil
// }

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
func CreateFlockFile(dir string) (*os.File, error) {
	flockName := filepath.Join(dir, "flock.lock")
	logger.Infof("file lock on %q", flockName)

	if err := Lock(flockName); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %s", flockName, err)
	}

	flockF, err := os.Open(flockName)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %s", flockName, err)
	}
	return flockF, nil
}

// MustGetFreeSpace returns free space for the given directory path.
func MustGetFreeSpace(path string) uint64 {
	lpFreeBytesAvailable := int64(0)
	lpTotalNumberOfBytes := int64(0)
	lpTotalNumberOfFreeBytes := int64(0)
	_, _, err := getFreeSpace.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(filepath.VolumeName(path)))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfFreeBytes)))

	if err.Error() != "Success." {
		fmt.Println("This is error:", err)
		logger.Panicf("FATAL: cannot  determine free disk space on %q: %#v", path, err)
	}
	return uint64(lpFreeBytesAvailable)
}
