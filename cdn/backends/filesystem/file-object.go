package filesystem

import (
	"sync"
	"time"
)

func NewFileObject(cachedSize uint, actualSize int64, modifiedTime time.Time) *FileObject {
	var ch []byte
	var cwi int
	if cachedSize == 0 {
		ch = nil
		cwi = 0
	} else {
		ch = make([]byte, cachedSize)
		cwi = -1
	}
	return &FileObject{
		cache:           ch,
		cacheWriteIndex: cwi,
		modifyTime:      modifiedTime,
		size:            actualSize,
		locker:          &sync.Mutex{},
	}
}

type FileObject struct {
	cache           []byte
	cacheWriteIndex int
	modifyTime      time.Time
	size            int64
	locker          *sync.Mutex
}

func (f *FileObject) doCache() bool {
	f.locker.Lock()
	defer f.locker.Unlock()
	if f.cacheWriteIndex < 0 {
		f.cacheWriteIndex = 0
		return true
	} else {
		return false
	}
}

func (f *FileObject) Write(p []byte) (n int, err error) {
	if f.cacheWriteIndex < len(f.cache) {
		if f.cacheWriteIndex+len(p) <= len(f.cache) {
			copy(f.cache[f.cacheWriteIndex:], p)
			f.cacheWriteIndex += len(p)
		} else {
			inputLen := len(f.cache) - f.cacheWriteIndex
			copy(f.cache[f.cacheWriteIndex:], p[0:inputLen])
			f.cacheWriteIndex += inputLen
		}
	}
	return len(p), nil
}
