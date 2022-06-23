package backends

import (
	"os"
	pth "path"
	"strconv"
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
		directoryPath: directory,
		fileStats:     fstats,
	}
}

type BackendFilesystem struct {
	directoryPath string
	fileStats     map[string]time.Time
}

func (b *BackendFilesystem) GetData(path string) []byte {
	if file, err := os.ReadFile(pth.Join(b.directoryPath, path)); err == nil {
		return file
	} else {
		return nil
	}
}

func (b *BackendFilesystem) Purge(path string) {
	_ = os.Remove(pth.Join(b.directoryPath, path))
}

func (b *BackendFilesystem) Exists(path string) bool {
	if _, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		return true
	} else {
		return false
	}
}

func (b *BackendFilesystem) Updated(path string) bool {
	if b.fileStats == nil {
		return true
	}
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		oldMTime := b.fileStats[path]
		defer func() {
			b.fileStats[path] = fstats.ModTime()
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
