package grace

import (
	"github.com/smartwalle/grace/gracenet"
	"net"
	"sync"
)

type Net struct {
	*options
	net     *gracenet.Net
	errChan chan error
}

func NewNet(opts ...option) *Net {
	var n = &Net{
		options: &options{restart: func() error { return nil }},
		net:     &gracenet.Net{},
		errChan: make(chan error, 1),
	}
	for _, opt := range opts {
		opt(n.options)
	}
	if n.wg == nil {
		n.wg = &sync.WaitGroup{}
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

func (n *Net) wait() {
	n.wg.Add(len(n.net.ActiveListeners()))
	go n.signalHandler()
	n.wg.Wait()
}

func (n *Net) term() {
	var lns = n.net.ActiveListeners()
	for _, ln := range lns {
		go func(ln net.Listener) {
			defer n.wg.Done()
			if err := ln.Close(); err != nil {
				n.errChan <- err
			}
		}(ln)
	}
}
