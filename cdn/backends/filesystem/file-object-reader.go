package filesystem

import (
	"errors"
	"io"
	"os"
)

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

func (r *FileObjectReader) Seek(offset int64, whence int) (int64, error) {
	if whence == 0 {
		if offset >= 0 && r.cacheReadIndex+offset < r.fileObject.size {
			r.cacheReadIndex = offset
		} else {
			return r.cacheReadIndex, errors.New("seek index out of range")
		}
	} else if whence == 1 {
		if r.cacheReadIndex+offset < 0 || r.cacheReadIndex+offset >= r.fileObject.size {
			return r.cacheReadIndex, errors.New("seek index out of range")
		} else {
			r.cacheReadIndex += offset
		}
	} else if whence == 2 {
		if r.fileObject.size+offset < 0 || r.fileObject.size+offset >= r.fileObject.size {
			return r.cacheReadIndex, errors.New("seek index out of range")
		} else {
			r.cacheReadIndex = r.fileObject.size + offset
		}
	} else {
		return r.cacheReadIndex, errors.New("invalid seek whence")
	}
	if r.filePointer != nil {
		return r.filePointer.Seek(offset, whence)
	}
	return r.cacheReadIndex, nil
}

func (r *FileObjectReader) Close() error {
	if r.filePointer != nil {
		return r.filePointer.Close()
	}
	return nil
}

func (r *FileObjectReader) Read(p []byte) (n int, err error) {
	cacheLen := r.fileObject.cacheWriteIndex
	if int(r.cacheReadIndex) < cacheLen {
		if int(r.cacheReadIndex)+len(p) <= cacheLen {
			copy(p, r.fileObject.cache[r.cacheReadIndex:r.cacheReadIndex+int64(len(p))])
			r.cacheReadIndex += int64(len(p))
			return len(p), nil
		} else {
			numRead := cacheLen - int(r.cacheReadIndex)
			copy(p, r.fileObject.cache[r.cacheReadIndex:cacheLen])
			r.cacheReadIndex = int64(cacheLen)
			return numRead, nil
		}
	} else if r.cacheReadIndex < r.fileObject.size {
		if r.filePointer == nil {
			nf, err := os.Open(r.filePath)
			if err != nil {
				return 0, err
			}
			if r.cacheReadIndex > 0 {
				_, err = nf.Seek(r.cacheReadIndex, 0)
				if err != nil {
					return 0, err
				}
			}
			r.filePointer = nf
		}
		read, err := r.filePointer.Read(p)
		if err != nil {
			return 0, err
		} else {
			r.cacheReadIndex += int64(read)
			return read, err
		}
	}
	return 0, io.EOF
}
