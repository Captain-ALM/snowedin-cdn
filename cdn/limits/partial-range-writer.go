package limits

import (
	"io"
	"snow.mrmelon54.xyz/snowedin/cdn/utils"
)

func NewPartialRangeWriter(writerIn io.Writer, httpRangeIn utils.ContentRangeValue) io.Writer {
	return &PartialRangeWriter{
		passedWriter:       writerIn,
		passedWriterIndex:  0,
		httpRange:          httpRangeIn,
		exclusiveLastIndex: httpRangeIn.Start + httpRangeIn.Length,
	}
}

type PartialRangeWriter struct {
	passedWriter       io.Writer
	passedWriterIndex  int64
	exclusiveLastIndex int64
	httpRange          utils.ContentRangeValue
}

func (prw *PartialRangeWriter) Write(p []byte) (n int, err error) {
	var pOffsetIndex int64 = -1
	if prw.passedWriterIndex >= prw.httpRange.Start && prw.passedWriterIndex < prw.exclusiveLastIndex {
		pOffsetIndex = 0
	} else if prw.passedWriterIndex+int64(len(p)) > prw.httpRange.Start && prw.passedWriterIndex < prw.exclusiveLastIndex {
		pOffsetIndex = prw.httpRange.Start - prw.passedWriterIndex
		prw.passedWriterIndex += pOffsetIndex
	} else {
		prw.passedWriterIndex += int64(len(p))
	}
	if pOffsetIndex >= 0 {
		if prw.passedWriterIndex+(int64(len(p))-pOffsetIndex) <= prw.exclusiveLastIndex {
			written, err := prw.passedWriter.Write(p[pOffsetIndex:])
			prw.passedWriterIndex += int64(written)
			if err != nil {
				return written, err
			}
		} else {
			written, err := prw.passedWriter.Write(p[pOffsetIndex : prw.exclusiveLastIndex-prw.passedWriterIndex+pOffsetIndex])
			prw.passedWriterIndex += int64(written)
			if err != nil {
				return written, err
			}
		}
	}
	return n, nil
}
