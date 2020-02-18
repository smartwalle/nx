package main

import (
	"fmt"
	"github.com/smartwalle/grace"
)

func main() {
	var n = grace.NewNet()
	ln, err := n.Listen("tcp", ":8899")
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
		n.AddConn(conn)

		go func() {
			conn.Write([]byte("hello"))
			n.RemoveConn(conn)
		}()
	}()

	n.Run()
}
