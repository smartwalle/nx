package main

import (
	"fmt"
	"github.com/smartwalle/grace"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	var wg = &sync.WaitGroup{}
	go grace.ServeWithOptions(
		[]*http.Server{{Addr: ":9900", Handler: newHandler()}},
		grace.WithWait(wg),
		grace.WithRestartHandler(func() error {
			fmt.Println("Restart")
			return nil
		}),
	)

	var c = make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			fmt.Println(time.Now(), "等待任务结束...")
			wg.Wait()
			fmt.Println(time.Now(), "任务完成，程序关闭。")
			return
		case syscall.SIGHUP:
		default:
			fmt.Println(time.Now(), "等待任务结束...")
			wg.Wait()
			fmt.Println(time.Now(), "任务完成，程序关闭。")
			return
		}
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello 2221 %d \n", os.Getpid())
		time.Sleep(time.Second * 10)
	})
	return mux
}
