// +build windows

package grace

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func (h *HTTP) signalHandler(wg *sync.WaitGroup) {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	for {
		sig := <-ch
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// this ensures a subsequent INT/TERM will trigger standard go behaviour of
			// terminating.
			signal.Stop(ch)
			h.term(wg)
		}
	}
}

func (h *HTTP) Run() error {
	if err := h.listen(); err != nil {
		return err
	}

	for i, s := range h.servers {
		go s.Serve(h.listeners[i])
	}

	h.serve()

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		h.wait()
	}()

	select {
	case err := <-h.errors:
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
