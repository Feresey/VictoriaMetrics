// +build windows

package netstorage

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"os"
	"syscall"
)

func mustFadviseSequentialRead(f *os.File) {
	filenameW, err := syscall.UTF16PtrFromString(f.Name())
	if  err != nil {
		logger.Panicf("FATAL: error conversing filename: %s", err)
	}
	err = syscall.SetFileAttributes(filenameW, syscall.FILE_SHARE_READ)
	if  err != nil {
		logger.Panicf("FATAL: error returned from syscall.SetFileAttributes: %s", err)
	}
}
