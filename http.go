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

type Waiter interface {
	Wait()
}

type options struct {
	restart func() error
	waiter  Waiter
}

type option func(*options)

// WithRestartHandler configures a callback to trigger during graceful restart
// directly before starting the successor process. This allows the current
// process to release holds on resources that the new process will need.
func WithRestartHandler(handler func() error) option {
	return func(opts *options) {
		opts.restart = handler
	}
}

func WithWait(w Waiter) option {
	return func(opts *options) {
		opts.waiter = w
	}
}

// An HTTP contains one or more servers and associated configuration.
type HTTP struct {
	*options
	servers   []*http.Server
	net       *gracenet.Net
	listeners []net.Listener
	errors    chan error
}

func NewHTTP(servers []*http.Server, opts ...option) *HTTP {
	var h = &HTTP{
		options:   &options{restart: func() error { return nil }},
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

func (h *HTTP) listen() error {
	for _, s := range h.servers {
		l, err := h.net.Listen("tcp", s.Addr)
		if err != nil {
			return err
		}
		if s.TLSConfig != nil {
			l = tls.NewListener(l, s.TLSConfig)
		}
		h.listeners = append(h.listeners, l)
	}
	return nil
}

func (h *HTTP) serve() {
	for i, s := range h.servers {
		go s.Serve(h.listeners[i])
	}
}

func (h *HTTP) wait() {
	var wg = &sync.WaitGroup{}
	wg.Add(len(h.servers)) // Wait & Stop
	go h.signalHandler(wg)
	//for _, s := range h.servers {
	//	s.RegisterOnShutdown(func() {
	//		defer wg.Done()
	//	})
	//}
	wg.Wait()
	if h.waiter != nil {
		h.waiter.Wait()
	}
}

func (h *HTTP) term(wg *sync.WaitGroup) {
	for _, s := range h.servers {
		go func(s *http.Server) {
			defer wg.Done()
			if err := s.Shutdown(context.Background()); err != nil {
				h.errors <- err
			}
		}(s)
	}
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
