package main

import (
	"fmt"
	"github.com/smartwalle/grace"
	"sync"
	"time"
)

func main() {
	var w = &sync.WaitGroup{}
	var n = grace.NewNet(grace.WithWait(w))
	ln, err := n.Listen("tcp", ":8891")
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		w.Add(1)
		go func() {
			for i := 0; i < 100; i++ {
				_, err := conn.Write([]byte("hello"))

				if err != nil {
					w.Done()
					return
				}
				time.Sleep(time.Second * 1)
			}
			w.Done()
		}()
	}()

	n.Run()
}
