// +build windows
package fs

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/juju/fslock"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
func CreateFlockFile(dir string) (*os.File, error) {
	logger.Infof("file lock dir %s", dir)
	flockFile := filepath.Join(dir, "flock.lock")
	winlock := fslock.New(flockFile)

	if err := winlock.Lock(); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %s", flockFile, err)
	}

	flockF, err := os.Open(flockFile)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %s", flockFile, err)
	}
	return flockF, nil
}

// MustGetFreeSpace returns free space for the given directory path.
func MustGetFreeSpace(path string) uint64 {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	lpFreeBytesAvailable := int64(0)
	lpTotalNumberOfBytes := int64(0)
	lpTotalNumberOfFreeBytes := int64(0)
	_, _, err := c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(filepath.VolumeName(path)))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfFreeBytes)))

	if err.Error() != "Success." {
		fmt.Println("This is error:",err)
		logger.Panicf("FATAL: cannot determine free disk space on %q: %#v", path, err)
	}
	return uint64(lpFreeBytesAvailable)
}
