package main

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

func keyboardEventsForward() chan []byte {
	keyboardEventsChan := make(chan []byte, 0)
	go func() {
		for event := range keyboardEventsChan {
			header := getMsgHeader(event)
			if header.opcode == waylandWlKeyboardKeyEventOpcode {
				ke, err := DecodeKeyEvent(event[8:])
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				fmt.Println(ke)
			} else if header.opcode == waylandWlKeyboardModifiersOpcode {
				km, err := DecodeKeyboardModifiersEvent(event[8:])
				if err != nil {
					fmt.Println(err)
					continue
				}
				fmt.Println(km)
			}
		}
	}()
	return keyboardEventsChan
}

func receiveFromWayland(fd int, waylandDataChan chan []byte) {
	for {
		waylandData := make([]byte, 4096)
		n, err := syscall.Read(fd, waylandData)
		if err != nil {
			fmt.Println(err)
		}
		if n == 0 {
			continue
		}
		waylandDataChan <- waylandData
	}
}

func handleWaylandMsgs(fd int, state *State, waylandDataChan chan []byte, keyboardEventsChan chan []byte) {
	for data := range waylandDataChan {
		for len(data) > 0 {
			header := getMsgHeader(data)
			if header.msgSize == 0 {
				break
			}
			// fmt.Printf("header=%+v\n", header)
			// fmt.Println(data[:header.msgSize])
			if header.objectId == state.wlRegistry && header.opcode == waylandWlRegistryEventGlobal {
				// get registry complete, bind to the interface
				waylandIface := getMsgInterface(data)
				switch string(waylandIface.iface[:waylandIface.ifaceLen-1]) { // interface name given by wayland includes a string terminator
				case "wl_compositor":
					state.wlCompositor = RegistryBind(fd, state.wlRegistry, waylandIface.name, waylandIface.ifaceWithPadding, waylandIface.ifaceLen, waylandIface.version)
				case "wl_shm":
					state.wlShm = RegistryBind(fd, state.wlRegistry, waylandIface.name, waylandIface.ifaceWithPadding, waylandIface.ifaceLen, waylandIface.version)
				case "xdg_wm_base":
					state.xdgWmBase = RegistryBind(fd, state.wlRegistry, waylandIface.name, waylandIface.ifaceWithPadding, waylandIface.ifaceLen, waylandIface.version)
				case "wl_seat":
					state.wlSeat = RegistryBind(fd, state.wlRegistry, waylandIface.name, waylandIface.ifaceWithPadding, waylandIface.ifaceLen, waylandIface.version)
				}
			}
			if header.objectId == waylandDisplayObjectId && header.opcode == waylandWlDisplayErrorEvent {
				fmt.Println("error happend" + string(data))
				return
			}
			if state.wlCompositor != 0 && state.wlShm != 0 && state.xdgWmBase != 0 && state.wlSurface == 0 {
				// this goes after binding to the interface
				state.wlSurface = CreateSurface(fd, state)
				state.xdgSurface = GetXdgSurface(fd, state)
				state.xdgToplevel = GetXdgSurfaceTopLevel(fd, state)
				SurfaceCommit(fd, state)
			}
			if header.objectId == state.xdgWmBase && header.opcode == waylandXdgWmBaseEventPing {
				SendWmBasePong(data[8:12], fd, state) //skip the header and go to the argument which is ping
			}
			if header.objectId == state.xdgSurface && header.opcode == waylandXdgSurfaceEventConfigure {
				SendSurfaceAckConfigure(data[8:12], fd, state) //skip the header and go to the argument which is configure
			}
			if header.objectId == state.wlShm && header.opcode == waylandShmPoolEventFormat {
				fmt.Printf("Shm Pool format: %d\n", binary.LittleEndian.Uint32(data[8:12]))
			}
			if header.objectId == state.xdgToplevel && header.opcode == waylandXdgToplevelEventConfigure {
				fmt.Println("top level configure")
			}
			if header.objectId == state.wlKeyboard {
				keyboardEventsChan <- data[:header.msgSize]
			}
			if header.objectId == state.xdgToplevel && header.opcode == waylandXdgToplevelEventClose {
				fmt.Println("EXIT")
				return
			}
			// fmt.Printf("State=%+v\n", state)
			data = data[header.msgSize:]
		}

		if state.wlSeat != 0 && state.wlKeyboard == 0 {
			// request wl_keyboard
			state.wlKeyboard = CreateKeyboard(fd, state)

		}
		if state.stateState == stateSurfaceAckedConfigure {
			if state.wlShmPool == 0 {
				nextId, err := CreateShmPool(fd, state)
				if err != nil {
					return
				}
				state.wlShmPool = nextId
			}
			fmt.Printf("%+v\n", state)
			if state.wlBuffer == 0 {
				state.wlBuffer = CreateShmPoolBuffer(fd, state)
			}
			render(fd, state)
			state.stateState = stateSurfaceAttached
		}
	}
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
	// defer unix.Close(fd)
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

func main() {
	waylandDataChan := make(chan []byte)
	keyboardEventsChan := keyboardEventsForward()
	fd, err := DisplayConnect()
	if err != nil {
		return
	}
	defer syscall.Close(fd)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		receiveFromWayland(fd, waylandDataChan)
	}()
	currentId := GetRegistry(fd)
	state := createState(currentId)
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleWaylandMsgs(fd, state, waylandDataChan, keyboardEventsChan)
	}()
	wg.Wait()
}
