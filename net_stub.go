// +build !windows

package grace

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

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
			err := n.restartHandler()
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
