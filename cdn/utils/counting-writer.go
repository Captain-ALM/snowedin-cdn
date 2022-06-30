package utils

type CountingWriter struct {
	Length int64
}

func (c *CountingWriter) Write(p []byte) (n int, err error) {
	c.Length += int64(len(p))
	return len(p), nil
}
