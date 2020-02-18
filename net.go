package nx

import (
	"fmt"
	"github.com/smartwalle/nx/gracenet"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

func (n *Net) Run() error {
	// Some useful logging.
	if logger != nil {
		if didInherit {
			if ppid == 1 {
				logger.Printf("Listening on init activated %s", pprintAddr(n.net.ActiveListeners()))
			} else {
				const msg = "Graceful handoff of %s with new pid %d and old pid %d"
				logger.Printf(msg, pprintAddr(n.net.ActiveListeners()), os.Getpid(), ppid)
			}
		} else {
			const msg = "Serving %s with pid %d"
			logger.Printf(msg, pprintAddr(n.net.ActiveListeners()), os.Getpid())
		}
	}

	if didInherit && ppid != 1 {
		if err := syscall.Kill(ppid, syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to close parent: %s", err)
		}
	}

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		n.wait()
	}()

	select {
	case err := <-n.errChan:
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

func (n *Net) signalHandler() {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)
	for {
		sig := <-ch
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// this ensures a subsequent INT/TERM will trigger standard go behaviour of
			// terminating.
			signal.Stop(ch)
			var lns = n.net.ActiveListeners()
			for _, ln := range lns {
				if err := ln.Close(); err != nil {
					n.errChan <- err
				}
			}
			close(n.termChan)
			return
		case syscall.SIGUSR2:
			err := n.restartProcess()
			if err != nil {
				n.errChan <- err
			}
			// we only return here if there's an error, otherwise the new process
			// will send us a TERM when it's ready to trigger the actual shutdown.
			if _, err := n.net.StartProcess(); err != nil {
				n.errChan <- err
			}
		}
	}
}
