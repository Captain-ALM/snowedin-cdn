package cdn

import "snow.mrmelon54.xyz/snowedin/cdn/backends"

type Backend interface {
	GetData(path string) []byte
	Purge(path string)
	Exists(path string) bool
	Updated(path string) bool
	List(path string) []string
}

func NewBackendFromName(name string, confMap map[string]string) Backend {
	if name == "filesystem" {
		return backends.NewBackendFilesystem(confMap)
	}
	return nil
}
