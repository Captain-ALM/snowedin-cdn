package cdn

import (
	"io"
	"snow.mrmelon54.xyz/snowedin/cdn/backends"
)

type Backend interface {
	OpenReader(path string) io.Reader
	CloseReader(reader io.Reader)
	Purge(path string)
	Exists(path string) bool
	Updated(path string) bool
	List(path string) []string
	Size(path string) int64
}

func NewBackendFromName(name string, confMap map[string]string) Backend {
	if name == "filesystem" {
		return backends.NewBackendFilesystem(confMap)
	}
	return nil
}
