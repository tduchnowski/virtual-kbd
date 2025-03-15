package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/bendahl/uinput"
)

func runServer(port int, kbd uinput.Keyboard) {
	slog.Info(fmt.Sprintf("starting a virtual-keyboard service on port %d", port))
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error(fmt.Sprintf("unable to start a virtual-keyboard server. Address: %s. Error: %s", addr, err.Error()))
		os.Exit(1)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("error while accepting connection")
			break
		}
		slog.Info("accepted connection from: " + conn.RemoteAddr().String())
		go handleConnection(conn, kbd)
	}
}

func handleConnection(conn net.Conn, kbd uinput.Keyboard) {
	defer func() {
		slog.Info("closing connection with " + conn.RemoteAddr().String())
		conn.Close()
	}()
	msg := make([]byte, 2)
	for {
		_, err := conn.Read(msg)
		if err == io.EOF {
			slog.Info("connection " + conn.RemoteAddr().String() + " closed by client")
			return
		}
		if err != nil {
			slog.Error(fmt.Sprintf("couldn't read from connection. error: %s", err.Error()))
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
	//create uinput device
	kbd, err := uinput.CreateKeyboard("/dev/uinput", []byte("virt-kbd"))
	if err != nil {
		slog.Error("couldn't create uinput device. Exiting")
		os.Exit(1)
	}
	defer kbd.Close()
	// run the server
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		runServer(3001, kbd)
	}()
	wg.Wait()
}
