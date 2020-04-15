package grace

import (
	"github.com/smartwalle/grace/gracenet"
	"net"
	"sync"
)

type Net struct {
	*options
	wg       *sync.WaitGroup
	net      *gracenet.Net
	termChan chan struct{}
	errChan  chan error
}

func NewNet(opts ...option) *Net {
	var n = &Net{
		options:  &options{restartHandler: func() error { return nil }},
		wg:       &sync.WaitGroup{},
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

func (n *Net) Retain() {
	n.wg.Add(1)
}

func (n *Net) Done() {
	n.wg.Done()
}

func (n *Net) wait() {
	go n.signalHandler()

	select {
	case <-n.termChan:
		n.wg.Wait()
	}
}
