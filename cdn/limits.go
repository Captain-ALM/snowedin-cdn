package cdn

import (
	"io"
	"snow.mrmelon54.xyz/snowedin/structure"
	"sync"
	"time"
)

func NewAccessLimit(conf structure.AccessLimitYaml) *AccessLimit {
	expr := time.Now().Add(conf.ExpireTime)
	if conf.ExpireTime.Seconds() < 1 {
		expr = time.Time{}
	}
	return &AccessLimit{
		ExpireTime:        expr,
		Gone:              false,
		AccessLimit:       conf.AccessLimit != 0,
		AccessesRemaining: conf.AccessLimit,
	}
}

type AccessLimit struct {
	ExpireTime        time.Time
	Gone              bool
	AccessLimit       bool
	AccessesRemaining uint
}

func (al *AccessLimit) isExpired() bool {
	if al.ExpireTime.IsZero() {
		return false
	} else {
		if al.ExpireTime.After(time.Now()) {
			return false
		} else {
			return true
		}
	}
}

func (al *AccessLimit) accessLimitReached() bool {
	if al.AccessLimit {
		if al.AccessesRemaining == 0 {
			return true
		} else {
			al.AccessesRemaining--
		}
	}
	return false
}

func NewRequestLimit(conf structure.LimitRequestsYaml) *RequestLimit {
	return &RequestLimit{
		expireTime:        time.Now().Add(conf.RequestRateInterval),
		requestsRemaining: conf.MaxRequests,
		limitConf:         conf,
		mu:                &sync.Mutex{},
	}
}

type RequestLimit struct {
	mu                *sync.Mutex
	expireTime        time.Time
	requestsRemaining uint
	limitConf         structure.LimitRequestsYaml
}

func (rl *RequestLimit) startRequest() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.expireTime.After(time.Now()) {
		if rl.requestsRemaining == 0 {
			return false
		} else {
			rl.requestsRemaining--
		}
	} else {
		rl.expireTime = time.Now().Add(rl.limitConf.RequestRateInterval)
		rl.requestsRemaining = rl.limitConf.MaxRequests - 1
	}
	return true
}

func NewConnectionLimit(conf structure.LimitConnectionYaml) *ConnectionLimit {
	return &ConnectionLimit{
		connectionsRemaining: conf.MaxConnections,
		limitConf:            conf,
		mu:                   &sync.Mutex{},
	}
}

type ConnectionLimit struct {
	mu                   *sync.Mutex
	connectionsRemaining uint
	limitConf            structure.LimitConnectionYaml
}

func (cl *ConnectionLimit) startConnection() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.connectionsRemaining == 0 {
		return false
	} else {
		cl.connectionsRemaining--
		return true
	}
}

func (cl *ConnectionLimit) stopConnection() {
	cl.mu.Lock()
	cl.connectionsRemaining++
	cl.mu.Unlock()
}

func GetLimitedBandwidthWriter(bly structure.BandwidthLimitYaml, targetWriter io.Writer) io.Writer {
	return &LimitedBandwidthWriter{
		passedWriter:      targetWriter,
		passedWriterIndex: 0,
		limiterSettings:   bly,
	}
}

type LimitedBandwidthWriter struct {
	passedWriter      io.Writer
	passedWriterIndex uint
	limiterSettings   structure.BandwidthLimitYaml
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
