package main

import (
	"fmt"
	"github.com/bendahl/uinput"
	"net"
	"os"
	"sync"
)

func runServer(port int, kbd uinput.Keyboard) {
	fmt.Println("Starting a server on port ", port)
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("unable to start a server. Error: ", err.Error())
		fmt.Println("exiting")
		os.Exit(1)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("error while accepting connection")
			continue
		}
		go handleConnection(conn, kbd)
	}
}

func handleConnection(conn net.Conn, kbd uinput.Keyboard) {
	defer conn.Close()
	msg := make([]byte, 2)
	for {
		_, err := conn.Read(msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		var scancode int = int(msg[0])
		if msg[1] == 0 {
			kbd.KeyUp(scancode)
		} else {
			kbd.KeyDown(scancode)
		}
	}
}

func main() {
	kbd, err := uinput.CreateKeyboard("/dev/uinput", []byte("virt-kbd"))
	if err != nil {
		fmt.Println("couldn't create uinput device. Exiting")
		os.Exit(1)
	}
	defer kbd.Close()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		runServer(3001, kbd)
	}()
	wg.Wait()
}
