package limits

import (
	"io"
	"snow.mrmelon54.xyz/snowedin/conf"
	"time"
)

func GetLimitedBandwidthWriter(bly conf.BandwidthLimitYaml, targetWriter io.Writer) io.Writer {
	return &LimitedBandwidthWriter{
		passedWriter:      targetWriter,
		passedWriterIndex: 0,
		limiterSettings:   bly,
	}
}

type LimitedBandwidthWriter struct {
	passedWriter      io.Writer
	passedWriterIndex uint
	limiterSettings   conf.BandwidthLimitYaml
}

func (lbw *LimitedBandwidthWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	var currentArrayIndex uint = 0
	for currentArrayIndex < uint(len(p)) {
		if currentArrayIndex+(lbw.limiterSettings.Bytes-lbw.passedWriterIndex) < uint(len(p)) {
			written, err := lbw.passedWriter.Write(p[currentArrayIndex : currentArrayIndex+(lbw.limiterSettings.Bytes-lbw.passedWriterIndex)])
			currentArrayIndex += uint(written)
			lbw.passedWriterIndex += uint(written)
			if err != nil {
				return int(currentArrayIndex), err
			}
		} else {
			written, err := lbw.passedWriter.Write(p[currentArrayIndex:])
			currentArrayIndex += uint(written)
			lbw.passedWriterIndex += uint(written)
			if err != nil {
				return int(currentArrayIndex), err
			}
		}

		if lbw.passedWriterIndex >= lbw.limiterSettings.Bytes {
			lbw.passedWriterIndex = lbw.passedWriterIndex - lbw.limiterSettings.Bytes
			time.Sleep(lbw.limiterSettings.Interval)
		}
	}
	return len(p), nil
}
