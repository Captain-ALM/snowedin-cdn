package limits

import (
	"snow.mrmelon54.xyz/snowedin/conf"
	"sync"
)

func NewConnectionLimit(conf conf.LimitConnectionYaml) *ConnectionLimit {
	return &ConnectionLimit{
		ConnectionsRemaining: conf.MaxConnections,
		LimitConf:            conf,
		mu:                   &sync.Mutex{},
	}
}

type ConnectionLimit struct {
	mu                   *sync.Mutex
	ConnectionsRemaining uint
	LimitConf            conf.LimitConnectionYaml
}

func (cl *ConnectionLimit) StartConnection() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.ConnectionsRemaining == 0 {
		return false
	} else {
		cl.ConnectionsRemaining--
		return true
	}
}

func (cl *ConnectionLimit) StopConnection() {
	cl.mu.Lock()
	cl.ConnectionsRemaining++
	cl.mu.Unlock()
}
