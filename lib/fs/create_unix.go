// +build !windows

package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

type Fslock struct {
	FileName string
	fd       *os.File
}

func (f *Fslock) Lock() error {
	flockF, err := os.Create(f.FileName)
	if err != nil {
		return fmt.Errorf("cannot create lock file %q: qs", f.FileName, err)
	}
	if err := unix.Flock(int(flockF.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("cannot acquire lock on file %q: %q", f.FileName, err)
	}
	return nil
}

func (f *Fslock) Unlock() error {
	f.fd.Close()
	return nil
}

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
func CreateFlockFile(dir string) (*Fslock, error) {
	f := &Fslock{FileName: filepath.Join(dir, "flock.lock")}
	return f, f.Lock()
}

// MustGetFreeSpace returns free space for the given directory path.
func MustGetFreeSpace(path string) uint64 {
	d, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}
	defer MustClose(d)

	fd := d.Fd()
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(fd), &stat); err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}
	freeSpace := uint64(stat.Bavail) * uint64(stat.Bsize)
	return freeSpace
}
