package cdn

import (
	"io"
	"snow.mrmelon54.xyz/snowedin/cdn/backends/filesystem"
	"time"
)

type Backend interface {
	WriteData(path string, rw io.Writer) (err error)
	WriteDataRange(path string, rw io.Writer, index int64, length int64) (err error)
	MimeType(path string) (mimetype string)
	ETag(path string) (eTag string)
	Stats(path string) (size int64, modified time.Time, err error)
	Purge(path string) (err error)
	Exists(path string) (exists bool, listable bool)
	List(path string) (entries []string, err error)
}

func NewBackendFromName(name string, confMap map[string]string) Backend {
	if name == "filesystem" {
		return filesystem.NewBackendFilesystem(confMap)
	}
	return nil
}
