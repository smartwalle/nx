package grace

import (
	"github.com/smartwalle/grace/gracenet"
	"net"
	"sync"
	"time"
)

type Net struct {
	*options
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	net      *gracenet.Net
	termChan chan struct{}
	errChan  chan error
}

func NewNet(opts ...option) *Net {
	var n = &Net{
		options:  &options{restartProcess: func() error { return nil }},
		conns:    make(map[net.Conn]struct{}),
		net:      &gracenet.Net{},
		termChan: make(chan struct{}, 1),
		errChan:  make(chan error, 1),
	}
	for _, opt := range opts {
		opt(n.options)
	}
	return n
}

func (n *Net) Listen(nett, laddr string) (net.Listener, error) {
	return n.net.Listen(nett, laddr)
}

func (n *Net) ListenTCP(nett string, laddr *net.TCPAddr) (*net.TCPListener, error) {
	return n.net.ListenTCP(nett, laddr)
}

func (n *Net) ListenUnix(nett string, laddr *net.UnixAddr) (*net.UnixListener, error) {
	return n.net.ListenUnix(nett, laddr)
}

func (n *Net) AddConn(c net.Conn) {
	if c != nil {
		n.mu.Lock()
		n.conns[c] = struct{}{}
		n.mu.Unlock()
	}
}

func (n *Net) RemoveConn(c net.Conn) {
	if c != nil {
		n.mu.Lock()
		delete(n.conns, c)
		n.mu.Unlock()
	}
}

func (n *Net) wait() {
	go n.signalHandler()

	select {
	case <-n.termChan:
		for {
			n.mu.Lock()
			var connLen = len(n.conns)
			n.mu.Unlock()

			if connLen == 0 {
				break
			}
			time.Sleep(time.Second * 2)
		}
	}
}
