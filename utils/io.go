package utils

import "io"

func MustClose(closer io.Closer) {
	_ = closer.Close()
}
