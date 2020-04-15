package main

import (
	"fmt"
	"github.com/smartwalle/grace"
	"time"
)

func main() {
	var n = grace.NewNet()
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

		n.Retain()
		go func() {
			for i := 0; i < 100; i++ {
				fmt.Println(conn.Write([]byte("hello")))
				time.Sleep(time.Second * 1)
			}
			n.Done()
		}()
	}()

	n.Run()
}
