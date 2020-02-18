// Package gracehttp provides easy to use graceful restart
// functionality for HTTP server.
package grace

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/smartwalle/grace/gracenet"
	"github.com/smartwalle/grace/httpdown"
)

var (
	didInherit = os.Getenv("LISTEN_FDS") != ""
	ppid       = os.Getppid()
)

type options struct {
	restartProcess func() error
}

type option func(*options)

// WithRestartHook configures a callback to trigger during graceful restart
// directly before starting the successor process. This allows the current
// process to release holds on resources that the new process will need.
func WithRestartHook(hook func() error) option {
	return func(opts *options) {
		opts.restartProcess = hook
	}
}

// An HTTP contains one or more servers and associated configuration.
type HTTP struct {
	*options
	servers   []*http.Server
	http      *httpdown.HTTP
	net       *gracenet.Net
	listeners []net.Listener
	sds       []httpdown.Server
	errors    chan error
}

func NewHTTP(servers []*http.Server, opts ...option) *HTTP {
	var h = &HTTP{
		options:   &options{restartProcess: func() error { return nil }},
		servers:   servers,
		http:      &httpdown.HTTP{},
		net:       &gracenet.Net{},
		listeners: make([]net.Listener, 0, len(servers)),
		sds:       make([]httpdown.Server, 0, len(servers)),
		// 2x num servers for possible Close or Stop errors + 1 for possible
		// StartProcess error.
		errors: make(chan error, 1+(len(servers)*2)),
	}
	for _, opt := range opts {
		opt(h.options)
	}
	return h
}

func (a *HTTP) listen() error {
	for _, s := range a.servers {
		// TODO: default addresses
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
		a.sds = append(a.sds, a.http.Serve(s, a.listeners[i]))
	}
}

func (a *HTTP) wait() {
	var wg sync.WaitGroup
	wg.Add(len(a.sds) * 2) // Wait & Stop
	go a.signalHandler(&wg)
	for _, s := range a.sds {
		go func(s httpdown.Server) {
			defer wg.Done()
			if err := s.Wait(); err != nil {
				a.errors <- err
			}
		}(s)
	}
	wg.Wait()
}

func (a *HTTP) term(wg *sync.WaitGroup) {
	for _, s := range a.sds {
		go func(s httpdown.Server) {
			defer wg.Done()
			if err := s.Stop(); err != nil {
				a.errors <- err
			}
		}(s)
	}
}

func (a *HTTP) signalHandler(wg *sync.WaitGroup) {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)
	for {
		sig := <-ch
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// this ensures a subsequent INT/TERM will trigger standard go behaviour of
			// terminating.
			signal.Stop(ch)
			a.term(wg)
			return
		case syscall.SIGUSR2:
			err := a.restartProcess()
			if err != nil {
				a.errors <- err
			}
			// we only return here if there's an error, otherwise the new process
			// will send us a TERM when it's ready to trigger the actual shutdown.
			if _, err := a.net.StartProcess(); err != nil {
				a.errors <- err
			}
		}
	}
}

func (a *HTTP) Run() error {
	// Acquire Listeners
	if err := a.listen(); err != nil {
		return err
	}

	// Some useful logging.
	if logger != nil {
		if didInherit {
			if ppid == 1 {
				logger.Printf("Listening on init activated %s", pprintAddr(a.listeners))
			} else {
				const msg = "Graceful handoff of %s with new pid %d and old pid %d"
				logger.Printf(msg, pprintAddr(a.listeners), os.Getpid(), ppid)
			}
		} else {
			const msg = "Serving %s with pid %d"
			logger.Printf(msg, pprintAddr(a.listeners), os.Getpid())
		}
	}

	// Start serving.
	a.serve()

	// Close the parent if we inherited and it wasn't init that started us.
	if didInherit && ppid != 1 {
		if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to close parent: %s", err)
		}
	}

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		a.wait()
	}()

	select {
	case err := <-a.errors:
		if err == nil {
			panic("unexpected nil error")
		}
		return err
	case <-waitDone:
		if logger != nil {
			logger.Printf("Exiting pid %d.", os.Getpid())
		}
		return nil
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
