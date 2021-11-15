package util

import (
	"errors"
	"os"
	"syscall"
	"time"
)

func GetFileTime(path string) (cTime time.Time, aTime time.Time, mTime time.Time, err error) {
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	if stat.Sys() != nil {
		attr := stat.Sys().(*syscall.Stat_t)
		if attr != nil {
			cTime = time.Unix(int64(attr.Atim.Sec), int64(attr.Ctim.Nsec))
			aTime = time.Unix(int64(attr.Atim.Sec), int64(attr.Atim.Nsec))
			mTime = time.Unix(int64(attr.Atim.Sec), int64(attr.Mtim.Nsec))
		}
	} else {
		err = errors.New("file sys info is nil")
	}
	return
}
