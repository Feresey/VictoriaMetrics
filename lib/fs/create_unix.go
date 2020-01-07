package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

// Fslock :
type Fslock struct {
	fd *os.File
}

// Lock :
func (f *Fslock) Lock() error {
	if err := unix.Flock(int(f.fd.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fmt.Errorf("cannot acquire lock on file %q: %q", f.fd.Name(), err)
	}
	return nil
}

// Unlock :
func (f *Fslock) Unlock() error {
	return f.fd.Close()
}

// FileName :
func (f *Fslock) FileName() string { return f.fd.Name() }

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
func CreateFlockFile(dir string) (*Fslock, error) {
	file := filepath.Join(dir, "flock.lock")
	flockF, err := os.Create(file)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %q", file, err)
	}
	f := &Fslock{fd: flockF}
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
