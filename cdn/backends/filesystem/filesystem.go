package filesystem

import (
	"crypto"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
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
		ecfo, _ = strconv.ParseBool(confMap["existsCheckCanFileOpen"])
	}
	var wmod = false
	if confMap["watchModified"] != "" {
		wmod, _ = strconv.ParseBool(confMap["watchModified"])
	}
	var mtbe = true
	if confMap["mimeTypeByExtension"] != "" {
		lmtbe, err := strconv.ParseBool(confMap["mimeTypeByExtension"])
		if err == nil {
			mtbe = lmtbe
		}
	}
	var dirl = false
	if confMap["listDirectories"] != "" {
		dirl, _ = strconv.ParseBool(confMap["listDirectories"])
	}
	var dmtc = false
	if confMap["directoryModifiedTimeCheck"] != "" {
		dmtc, _ = strconv.ParseBool(confMap["directoryModifiedTimeCheck"])
	}
	var etagstore map[string]string = nil
	if confMap["calculateETags"] != "" {
		calcETags, _ := strconv.ParseBool(confMap["calculateETags"])
		if calcETags {
			etagstore = make(map[string]string)
		}
	}
	return &BackendFilesystem{
		directoryPath:              directory,
		cachedHeaderBytes:          chb,
		existsCheckFileOpen:        ecfo,
		watchModified:              wmod,
		mimeTypeByExtension:        mtbe,
		directoryListing:           dirl,
		directoryModifiedTimeCheck: dmtc,
		calculateETags:             etagstore != nil,
		fileObjects:                make(map[string]*FileObject),
		eTags:                      etagstore,
		syncer:                     &sync.Mutex{},
	}
}

type BackendFilesystem struct {
	directoryPath              string
	cachedHeaderBytes          uint
	existsCheckFileOpen        bool
	watchModified              bool
	mimeTypeByExtension        bool
	directoryListing           bool
	directoryModifiedTimeCheck bool
	calculateETags             bool
	fileObjects                map[string]*FileObject
	eTags                      map[string]string
	syncer                     *sync.Mutex
}

func (b *BackendFilesystem) ETag(path string) (eTag string) {
	if b.calculateETags {
		b.syncer.Lock()
		defer b.syncer.Unlock()
		if _, ok := b.eTags[path]; !ok {
			_, _, _ = b.directStats(path)
		}
		return b.eTags[path]
	}
	return ""
}

func (b *BackendFilesystem) MimeType(path string) (mimetype string) {
	pext := pth.Ext(path)
	if b.mimeTypeByExtension && pext != "" {
		return mime.TypeByExtension(pext)
	} else {
		return ""
	}
}

func (b *BackendFilesystem) WriteDataRange(path string, rw io.Writer, index int64, length int64) (err error) {
	fobj, err := b.getFileObject(path)
	if fobj == nil {
		return err
	} else {
		if fobj.size < 0 {
			return errors.New("object not writeable")
		}
		if len(fobj.cache) > 0 && fobj.doCache() {
			theFile, err := os.Open(pth.Join(b.directoryPath, path))
			if err != nil {
				return err
			}
			defer theFile.Close()
			_, err = io.Copy(fobj, io.LimitReader(theFile, int64(len(fobj.cache))))
			if err != nil {
				return err
			}
			theReader := &FileObjectReader{
				filePath:       pth.Join(b.directoryPath, path),
				fileObject:     fobj,
				cacheReadIndex: 0,
				filePointer:    theFile,
			}
			_, err = theReader.Seek(index, 0)
			if err != nil {
				return err
			}
			_, err = io.Copy(rw, io.LimitReader(theReader, length))
			if err != nil {
				return err
			}
		} else {
			theReader := NewFileObjectReader(pth.Join(b.directoryPath, path), fobj)
			defer theReader.Close()
			_, err = theReader.Seek(index, 0)
			if err != nil {
				return err
			}
			_, err = io.Copy(rw, io.LimitReader(theReader, length))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BackendFilesystem) WriteData(path string, rw io.Writer) (err error) {
	fobj, err := b.getFileObject(path)
	if fobj == nil {
		return err
	} else {
		if fobj.size < 0 {
			return errors.New("object not writeable")
		}
		if len(fobj.cache) > 0 && fobj.doCache() {
			theReader, err := os.Open(pth.Join(b.directoryPath, path))
			if err != nil {
				return err
			}
			defer theReader.Close()
			_, err = io.Copy(io.MultiWriter(rw, fobj), theReader)
			if err != nil {
				return err
			}
		} else {
			theReader := NewFileObjectReader(pth.Join(b.directoryPath, path), fobj)
			defer theReader.Close()
			_, err = io.Copy(rw, theReader)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BackendFilesystem) Stats(path string) (size int64, modified time.Time, err error) {
	fobj, err := b.getFileObject(path)
	if fobj == nil {
		return 0, time.Time{}, err
	} else {
		return fobj.size, fobj.modifyTime.UTC(), nil
	}
}

func (b *BackendFilesystem) setETag(path string, tagValue string, replaceExisting bool) {
	b.syncer.Lock()
	if replaceExisting || b.eTags[path] == "" {
		theHash := crypto.SHA1.New()
		_, _ = theHash.Write([]byte(tagValue))
		theSum := theHash.Sum(nil)
		theHash.Reset()
		if len(theSum) > 0 {
			b.eTags[path] = "\"" + hex.EncodeToString(theSum) + "\""
		} else {
			b.eTags[path] = "\"" + hex.EncodeToString([]byte(tagValue)) + "\""
		}
	}
	b.syncer.Unlock()
}

func (b *BackendFilesystem) getFileObject(path string) (*FileObject, error) {
	b.syncer.Lock()
	defer b.syncer.Unlock()
	if b.watchModified && b.fileObjects[path] != nil {
		sz, tm, err := b.directStats(path)
		if err == nil && (sz != b.fileObjects[path].size || !tm.Equal(b.fileObjects[path].modifyTime)) {
			b.fileObjects[path] = nil
		} else if err != nil {
			return nil, err
		}
	}
	if b.fileObjects[path] == nil {
		sz, tm, err := b.directStats(path)
		if err == nil {
			if sz < 0 {
				if !b.directoryModifiedTimeCheck {
					tm = time.Time{}
				}
				return &FileObject{
					cache:           nil,
					cacheWriteIndex: 0,
					modifyTime:      tm,
					size:            -1,
				}, nil
			} else {
				b.fileObjects[path] = NewFileObject(b.cachedHeaderBytes, sz, tm)
			}
		} else {
			return nil, err
		}
	}
	return b.fileObjects[path], nil
}

func (b *BackendFilesystem) directStats(path string) (size int64, modified time.Time, err error) {
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		if fstats.IsDir() {
			if b.calculateETags {
				b.setETag(path, "-1:"+fstats.ModTime().Format(http.TimeFormat), false)
			}
			return -1, fstats.ModTime(), nil
		} else {
			if b.calculateETags {
				b.setETag(path, strconv.FormatInt(fstats.Size(), 10)+":"+fstats.ModTime().Format(http.TimeFormat), false)
			}
			return fstats.Size(), fstats.ModTime(), nil
		}
	} else {
		return 0, time.Time{}, err
	}
}

func (b *BackendFilesystem) Purge(path string) (err error) {
	b.syncer.Lock()
	if _, ok := b.fileObjects[path]; ok {
		b.fileObjects[path] = nil
	}
	if b.calculateETags {
		if _, ok := b.eTags[path]; ok {
			b.eTags[path] = ""
		}
	}
	b.syncer.Unlock()
	return nil
}

func (b *BackendFilesystem) Exists(path string) (exists bool, listable bool) {
	if fstats, err := os.Stat(pth.Join(b.directoryPath, path)); err == nil {
		if fstats.IsDir() {
			return b.directoryListing, true
		} else {
			if b.existsCheckFileOpen {
				if file, err := os.Open(pth.Join(b.directoryPath, path)); err == nil {
					_ = file.Close()
					return true, false
				}
			} else {
				return true, false
			}
		}
	}
	return false, false
}

func (b *BackendFilesystem) List(path string) (entries []string, err error) {
	if dir, err := os.ReadDir(pth.Join(b.directoryPath, path)); err == nil {
		contents := make([]string, len(dir))
		for i, d := range dir {
			contents[i] = d.Name()
		}
		return contents, nil
	} else {
		return nil, err
	}
}
