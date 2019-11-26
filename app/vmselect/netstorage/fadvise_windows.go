// + build windows
package netstorage

import (
	"os"
)

func mustFadviseRandomRead(f *os.File) {
}
