// +build windows

package grace

import (
	"os"
	"os/signal"
	"syscall"
)

func (n *Net) signalHandler() {
	ch := make(chan os.Signal, 10)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	for {
		sig := <-ch
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			signal.Stop(ch)
			n.term()
			return
		}
	}
}

func (n *Net) Run() error {
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
