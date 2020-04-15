// Package gracehttp provides easy to use graceful restart
// functionality for HTTP server.
package grace

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/smartwalle/grace/gracenet"
	"net"
	"net/http"
	"os"
	"sync"
)

var (
	didInherit = os.Getenv("LISTEN_FDS") != ""
	ppid       = os.Getppid()
)

type options struct {
	restartHandler func() error
}

type option func(*options)

// WithRestartHandler configures a callback to trigger during graceful restart
// directly before starting the successor process. This allows the current
// process to release holds on resources that the new process will need.
func WithRestartHandler(handler func() error) option {
	return func(opts *options) {
		opts.restartHandler = handler
	}
}

// An HTTP contains one or more servers and associated configuration.
type HTTP struct {
	*options
	wg        *sync.WaitGroup
	servers   []*http.Server
	net       *gracenet.Net
	listeners []net.Listener
	errors    chan error
}

func NewHTTP(servers []*http.Server, opts ...option) *HTTP {
	var h = &HTTP{
		options:   &options{restartHandler: func() error { return nil }},
		wg:        &sync.WaitGroup{},
		servers:   servers,
		net:       &gracenet.Net{},
		listeners: make([]net.Listener, 0, len(servers)),
		errors:    make(chan error, 1+(len(servers)*2)),
	}
	for _, opt := range opts {
		opt(h.options)
	}
	return h
}

func (a *HTTP) listen() error {
	for _, s := range a.servers {
		l, err := a.net.Listen("tcp", s.Addr)
		if err != nil {
			return err
		}
		if s.TLSConfig != nil {
			l = tls.NewListener(l, s.TLSConfig)
		}
		a.listeners = append(a.listeners, l)
	}
	return nil
}

func (a *HTTP) serve() {
	for i, s := range a.servers {
		go s.Serve(a.listeners[i])
	}
}

func (a *HTTP) wait() {
	var wg = &sync.WaitGroup{}
	wg.Add(len(a.servers) * 2) // Wait & Stop
	go a.signalHandler(wg)
	for _, s := range a.servers {
		s.RegisterOnShutdown(func() {
			defer wg.Done()
		})
	}
	wg.Wait()
	a.wg.Wait()
}

func (a *HTTP) term(wg *sync.WaitGroup) {
	for _, s := range a.servers {
		go func(s *http.Server) {
			defer wg.Done()
			if err := s.Shutdown(context.Background()); err != nil {
				a.errors <- err
			}
		}(s)
	}
}

func (a *HTTP) Retain() {
	a.wg.Add(1)
}

func (a *HTTP) Done() {
	a.wg.Done()
}

// ServeWithOptions does the same as Serve, but takes a set of options to
// configure the HTTP struct.
func ServeWithOptions(servers []*http.Server, options ...option) error {
	var h = NewHTTP(servers, options...)
	return h.Run()
}

// Serve will serve the given http.Servers and will monitor for signals
// allowing for graceful termination (SIGTERM) or restart (SIGUSR2).
func Serve(servers ...*http.Server) error {
	var h = NewHTTP(servers)
	return h.Run()
}

// Used for pretty printing addresses.
func pprintAddr(listeners []net.Listener) []byte {
	var out bytes.Buffer
	for i, l := range listeners {
		if i != 0 {
			fmt.Fprint(&out, ", ")
		}
		fmt.Fprint(&out, l.Addr())
	}
	return out.Bytes()
}
