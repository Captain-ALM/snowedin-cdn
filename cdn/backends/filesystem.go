package backends

import (
	"io"
	"os"
	pth "path"
	"strconv"
	"sync"
	"time"
)

func NewBackendFilesystem(confMap map[string]string) *BackendFilesystem {
	wdir, _ := os.Getwd()
	directory := wdir
	if confMap["directoryPath"] != "" {
		directory = confMap["directoryPath"]
		fstat, err := os.Stat(directory)
		if err != nil || !fstat.IsDir() {
			directory = wdir
		}
	}
	var fstats map[string]time.Time
	if confMap["watchModifiedTime"] != "" {
		wmt, _ := strconv.ParseBool(confMap["watchModifiedTime"])
		if wmt {
			fstats = make(map[string]time.Time)
		}
	}
	return &BackendFilesystem{
		directoryPath:   directory,
		fileModifyStats: fstats,
		fileSizeStats:   make(map[string]int64),
		filePointers:    make([]*os.File, 0),
	}
}

type BackendFilesystem struct {
	directoryPath   string
	fileModifyStats map[string]time.Time
	fileSizeStats   map[string]int64
	filePointers    []*os.File
	syncer          sync.Mutex
}

func (b *BackendFilesystem) Size(path string) int64 {
	if val, ok := b.fileSizeStats[path]; ok {
		return val
	}
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		b.fileSizeStats[path] = fstats.Size()
		return b.fileSizeStats[path]
	}
	return 0
}

func (b *BackendFilesystem) OpenReader(path string) io.Reader {
	b.syncer.Lock()
	defer b.syncer.Unlock()
	if file, err := os.Open(pth.Join(b.directoryPath, path)); err == nil {
		b.filePointers = append(b.filePointers, file)
		return file
	} else {
		return nil
	}
}

func (b *BackendFilesystem) CloseReader(reader io.Reader) {
	if reader == nil {
		return
	}
	b.syncer.Lock()
	defer b.syncer.Unlock()
	targetIndex := -1
	for i, p := range b.filePointers {
		if p == reader {
			targetIndex = i
			_ = p.Close()
			return
		}
	}
	if targetIndex == -1 {
		return
	}
	b.filePointers[targetIndex] = b.filePointers[len(b.filePointers)-1]
	b.filePointers = b.filePointers[:len(b.filePointers)-1]
}

func (b *BackendFilesystem) Purge(path string) {
	_ = os.Remove(pth.Join(b.directoryPath, path))
}

func (b *BackendFilesystem) Exists(path string) bool {
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		if !fstats.IsDir() {
			if file, err := os.Open(pth.Join(b.directoryPath, path)); err == nil {
				_ = file.Close()
				return true
			}
		}
	}
	return false
}

func (b *BackendFilesystem) Updated(path string) bool {
	if b.fileModifyStats == nil {
		return true
	}
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		oldMTime := b.fileModifyStats[path]
		delete(b.fileSizeStats, path)
		defer func() {
			b.fileModifyStats[path] = fstats.ModTime()
		}()
		return !fstats.ModTime().Equal(oldMTime)
	} else {
		return false
	}
}

func (b *BackendFilesystem) List(path string) []string {
	if dir, err := os.ReadDir(pth.Join(b.directoryPath, path)); err == nil {
		contents := make([]string, len(dir))
		for i, d := range dir {
			contents[i] = pth.Join(b.directoryPath, path, d.Name())
		}
		return contents
	} else {
		return nil
	}
}
