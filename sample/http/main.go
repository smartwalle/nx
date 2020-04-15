package main

import (
	"fmt"
	"github.com/smartwalle/grace"
	"net/http"
	"os"
)

func main() {
	grace.ServeWithOptions(
		[]*http.Server{{Addr: ":9900", Handler: newHandler()}},
		grace.WithRestartHandler(func() error {
			fmt.Println("Restart")
			return nil
		}),
	)
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello 221 %d \n", os.Getpid())
	})
	return mux
}
