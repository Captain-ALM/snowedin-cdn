package backends

import (
	"io"
	"mime"
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
	var chb uint
	if confMap["cachedHeaderBytes"] != "" {
		lchb, err := strconv.ParseUint(confMap["cachedHeaderBytes"], 10, 32)
		if err == nil {
			chb = uint(lchb)
		}
	}
	var ecfo = false
	if confMap["existsCheckCanFileOpen"] != "" {
		ecfo, _ = strconv.ParseBool("existsCheckCanFileOpen")
	}
	return &BackendFilesystem{
		directoryPath:       directory,
		cachedHeaderBytes:   chb,
		existsCheckFileOpen: ecfo,
		fileObjects:         make(map[string]*FileObject),
	}
}

type BackendFilesystem struct {
	directoryPath       string
	cachedHeaderBytes   uint
	existsCheckFileOpen bool
	fileObjects         map[string]*FileObject
	syncer              sync.Mutex
}

func (b *BackendFilesystem) MimeType(path string) (mimetype string) {
	return mime.TypeByExtension(path)
}

func (b *BackendFilesystem) WriteData(path string, rw io.Writer) (err error) {
	fobj, err := b.getFileObject(path)
	if fobj == nil {
		return err
	} else {
		multWriter := io.MultiWriter(rw, fobj)
		fobjReader := NewFileObjectReader(pth.Join(b.directoryPath, path), fobj)
		defer fobjReader.Close()
		_, err := io.Copy(multWriter, fobjReader)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *BackendFilesystem) Stats(path string) (size int64, modified time.Time, err error) {
	fobj, err := b.getFileObject(path)
	if fobj == nil {
		return 0, time.Time{}, err
	} else {
		return fobj.size, fobj.modifyTime, nil
	}
}

func (b *BackendFilesystem) getFileObject(path string) (*FileObject, error) {
	b.syncer.Lock()
	defer b.syncer.Unlock()
	if b.fileObjects[path] == nil {
		sz, tm, err := b.directStats(path)
		if err == nil {
			b.fileObjects[path] = NewFileObject(b.cachedHeaderBytes, sz, tm)
		} else {
			return nil, err
		}
	}
	return b.fileObjects[path], nil
}

func (b *BackendFilesystem) directStats(path string) (size int64, modified time.Time, err error) {
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		return fstats.Size(), fstats.ModTime(), nil
	} else {
		return 0, time.Time{}, err
	}
}

func (b *BackendFilesystem) Purge(path string) (err error) {
	return os.Remove(pth.Join(b.directoryPath, path))
}

func (b *BackendFilesystem) Exists(path string) bool {
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		if !fstats.IsDir() {
			if b.existsCheckFileOpen {
				if file, err := os.Open(pth.Join(b.directoryPath, path)); err == nil {
					_ = file.Close()
					return true
				}
			} else {
				return true
			}
		}
	}
	return false
}

func (b *BackendFilesystem) List(path string) (entries []string, err error) {
	if dir, err := os.ReadDir(pth.Join(b.directoryPath, path)); err == nil {
		contents := make([]string, len(dir))
		for i, d := range dir {
			contents[i] = pth.Join(b.directoryPath, path, d.Name())
		}
		return contents, nil
	} else {
		return nil, err
	}
}

func NewFileObject(cachedSize uint, actualSize int64, modifiedTime time.Time) *FileObject {
	return &FileObject{
		cache:           make([]byte, cachedSize),
		cacheWriteIndex: 0,
		modifyTime:      modifiedTime,
		size:            actualSize,
	}
}

type FileObject struct {
	cache           []byte
	cacheWriteIndex int
	modifyTime      time.Time
	size            int64
}

func (fobj *FileObject) Write(p []byte) (n int, err error) {
	if fobj.cacheWriteIndex < len(fobj.cache) {
		if fobj.cacheWriteIndex+len(p) <= len(fobj.cache) {
			copy(fobj.cache[fobj.cacheWriteIndex:], p)
		} else {
			copy(fobj.cache[fobj.cacheWriteIndex:], p[0:len(fobj.cache)-fobj.cacheWriteIndex])
		}
		fobj.cacheWriteIndex += len(p)
	}
	return len(p), nil
}

func (fobj *FileObject) cacheWritten() bool {
	return fobj.cacheWriteIndex >= len(fobj.cache)
}

func NewFileObjectReader(targetPath string, fObj *FileObject) *FileObjectReader {
	return &FileObjectReader{
		filePath:       targetPath,
		fileObject:     fObj,
		cacheReadIndex: 0,
		filePointer:    nil,
	}
}

type FileObjectReader struct {
	filePath       string
	fileObject     *FileObject
	cacheReadIndex int64
	filePointer    *os.File
}

func (fobjr *FileObjectReader) Close() error {
	if fobjr.filePointer != nil {
		return fobjr.filePointer.Close()
	}
	return nil
}

func (fobjr *FileObjectReader) Read(p []byte) (n int, err error) {
	if int(fobjr.cacheReadIndex) < len(fobjr.fileObject.cache) {
		if int(fobjr.cacheReadIndex)+len(p) <= len(fobjr.fileObject.cache) {
			copy(p, fobjr.fileObject.cache[fobjr.cacheReadIndex:fobjr.cacheReadIndex+int64(len(p))])
			fobjr.cacheReadIndex += int64(len(p))
			return len(p), nil
		} else {
			numRead := len(fobjr.fileObject.cache) - int(fobjr.cacheReadIndex)
			copy(p, fobjr.fileObject.cache[fobjr.cacheReadIndex:len(fobjr.fileObject.cache)])
			fobjr.cacheReadIndex = int64(len(fobjr.fileObject.cache))
			return numRead, nil
		}
	} else if fobjr.cacheReadIndex < fobjr.fileObject.size {
		if fobjr.filePointer == nil {
			nf, err := os.Open(fobjr.filePath)
			if err != nil {
				return 0, err
			}
			_, err = nf.Seek(fobjr.cacheReadIndex, 0)
			if err != nil {
				return 0, err
			}
			fobjr.filePointer = nf
		}
		read, err := fobjr.filePointer.Read(p)
		if err != nil {
			return 0, err
		} else {
			fobjr.cacheReadIndex += int64(read)
			return read, err
		}
	}
	return 0, io.EOF
}
