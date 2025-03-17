package main

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func keyboardEventsForward(targetConn *net.TCPConn, done chan bool) chan []byte {
	keyboardEventsChan := make(chan []byte, 0)
	go func() {
		for event := range keyboardEventsChan {
			slog.Debug(fmt.Sprintf("received data: %v", event))
			header := getMsgHeader(event)
			if header.opcode == waylandWlKeyboardKeyEventOpcode {
				ke, err := DecodeKeyEvent(event[8:])
				if err != nil {
					slog.Error("while decoding keyboard data: " + err.Error())
					continue
				}
				keyMsg := make([]byte, 2)
				keyMsg[0] = byte(ke.scanCode)
				if ke.state {
					keyMsg[1] = byte(1)
				} else {
					keyMsg[1] = byte(0)
				}
				slog.Info(fmt.Sprintf("sending %v", keyMsg))
				_, err = targetConn.Write(keyMsg)
				if err != nil {
					slog.Error(err.Error())
					if errors.Is(err, syscall.EPIPE) {
						done <- true
					}
				}
			}
		}
	}()
	return keyboardEventsChan
}

func receiveFromWayland(fd int, state *State, keyboardEvents chan []byte, done chan bool) {
	for {
		waylandData := make([]byte, 4096)
		n, err := syscall.Read(fd, waylandData)
		if err != nil {
			slog.Error("while reading from a socket: " + err.Error())
			if errors.Is(err, syscall.EPIPE) {
				done <- true
			}
			if n == 0 {
				continue
			}
		}
		handleWaylandData(fd, state, keyboardEvents, waylandData, done)
	}
}

func handleWaylandData(fd int, state *State, keyboardEvents chan []byte, data []byte, done chan bool) {
	for len(data) > 0 {
		header := getMsgHeader(data)
		if header.msgSize == 0 {
			break
		}
		if header.objectId == state.wlRegistry && header.opcode == waylandWlRegistryEventGlobal {
			bindInterface(fd, state, data)
		} else if header.objectId == waylandDisplayObjectId && header.opcode == waylandWlDisplayErrorEvent {
			slog.Error("display server sent an error. closing")
			slog.Debug(fmt.Sprintf("state: %+v\n", state))
			done <- true
		} else if header.objectId == state.xdgWmBase && header.opcode == waylandXdgWmBaseEventPing {
			SendWmBasePong(data, fd, state)
		} else if header.objectId == state.xdgSurface && header.opcode == waylandXdgSurfaceEventConfigure {
			SendSurfaceAckConfigure(data, fd, state)
		} else if header.objectId == state.wlKeyboard {
			keyboardEvents <- data[:header.msgSize]
		} else if header.objectId == state.xdgToplevel && header.opcode == waylandXdgToplevelEventClose {
			slog.Info("top level event close received. exiting")
			done <- true
		}
		data = data[header.msgSize:]
	}
	if state.wlCompositor != 0 && state.wlShm != 0 && state.xdgWmBase != 0 && state.wlSurface == 0 {
		wlSurfaceSetup(fd, state)
	}
	if state.wlSeat != 0 && state.wlKeyboard == 0 {
		state.wlKeyboard = CreateKeyboard(fd, state)
	}
	if state.wlSeat != 0 && state.zwpShortcutsInhibitor == 0 {
		state.zwpShortcutsInhibitor = InhibitGlobalShortcuts(fd, state)
	}
	if state.stateState == stateSurfaceAckedConfigure {
		configureSurface(fd, state)
	}
}

func bindInterface(fd int, state *State, data []byte) {
	waylandIface := getMsgInterface(data)
	var err error
	switch string(waylandIface.iface[:waylandIface.len-1]) { // interface name given by wayland includes a string terminator
	case "wl_compositor":
		state.wlCompositor, err = RegistryBind(fd, state.wlRegistry, waylandIface)
	case "wl_shm":
		state.wlShm, err = RegistryBind(fd, state.wlRegistry, waylandIface)
	case "xdg_wm_base":
		state.xdgWmBase, err = RegistryBind(fd, state.wlRegistry, waylandIface)
	case "wl_seat":
		state.wlSeat, err = RegistryBind(fd, state.wlRegistry, waylandIface)
	case "zwp_keyboard_shortcuts_inhibit_manager_v1":
		state.zwpShortcutsInhibitMngr, err = RegistryBind(fd, state.wlRegistry, waylandIface)
	}
	if err != nil {
		slog.Error(err.Error())
	}
}

func wlSurfaceSetup(fd int, state *State) {
	slog.Debug("setting up wl_surface")
	state.wlSurface = CreateSurface(fd, state)
	state.xdgSurface = GetXdgSurface(fd, state)
	state.xdgToplevel = GetXdgSurfaceTopLevel(fd, state)
	SurfaceCommit(fd, state)
}

func configureSurface(fd int, state *State) {
	slog.Debug("configuring surface")
	if state.wlShmPool == 0 {
		nextId, err := CreateShmPool(fd, state)
		if err != nil {
			return
		}
		state.wlShmPool = nextId
	}
	if state.wlBuffer == 0 {
		state.wlBuffer = CreateShmPoolBuffer(fd, state)
	}
	render(fd, state)
	state.stateState = stateSurfaceAttached
}

func render(fd int, state *State) {
	start := rand.IntN(int(state.shmPoolSize))
	for j := 0; j < start; j++ {
		(*state.shmPoolData)[j] = byte(0)
	}
	for i := start; i < int(state.shmPoolSize); i++ {
		(*state.shmPoolData)[i] = byte(rand.IntN(100))
	}
	SurfaceAttach(fd, state)
	SurfaceCommit(fd, state)
}

func createState(currentId uint32) *State {
	state := State{
		wlRegistry: currentId,
		w:          700,
		h:          700,
	}
	state.stride = state.w * colorChannels
	state.shmPoolSize = state.h * state.stride
	fd, err := unix.MemfdCreate("wayland_shared_mem", 0)
	if err != nil {
		fmt.Println(err)
		return &State{}
	}
	err = unix.Ftruncate(fd, int64(state.shmPoolSize))
	if err != nil {
		fmt.Println(err)
		return &State{}
	}
	data, err := unix.Mmap(fd, 0, int(state.shmPoolSize), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	state.shmPoolData = &data
	state.shmFd = fd
	return &state
}

func connectToRemote() (*net.TCPConn, error) {
	connType := "tcp"
	host := os.Args[1]
	port := os.Args[2]
	serv := fmt.Sprintf("%s:%s", host, port)
	tcpServer, err := net.ResolveTCPAddr(connType, serv)
	if err != nil {
		return nil, err
	}
	return net.DialTCP(connType, nil, tcpServer)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("provide target machine ip and port, eg. 192.168.124.3 3001")
		return
	}
	if os.Getenv("DEBUG") == "1" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	conn, err := connectToRemote()
	if err != nil {
		slog.Error("couldn't connect to the target machine: " + err.Error())
		return
	}
	defer conn.Close()
	fd, err := DisplayConnect()
	if err != nil {
		slog.Error(err.Error())
		return
	}
	defer syscall.Close(fd)
	currentId := GetRegistry(fd)
	state := createState(currentId)
	done := make(chan bool)
	keyboardEventsChan := keyboardEventsForward(conn, done)
	go receiveFromWayland(fd, state, keyboardEventsChan, done)
	<-done
}
