package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
)

var waylandCurrentId uint32 = 1

const waylandDisplayObjectId uint32 = 1
const waylandWlRegistryEventGlobal uint16 = 0
const waylandShmPoolEventFormat uint16 = 0
const waylandWlBufferEventRelease uint16 = 0
const waylandXdgWmBaseEventPing uint16 = 0
const waylandXdgToplevelEventConfigure uint16 = 0
const waylandXdgToplevelEventClose uint16 = 1
const waylandXdgSurfaceEventConfigure uint16 = 0
const waylandWlDisplayGetRegistryOpcode uint16 = 1
const waylandWlRegistryBindOpcode uint16 = 0
const waylandWlCompositorCreateSurfaceOpcode uint16 = 0
const waylandXdgWmBasePongOpcode uint16 = 3
const waylandXdgSurfaceAckConfigureOpcode uint16 = 4
const waylandWlShmCreatePoolOpcode uint16 = 0
const waylandXdgWmBaseGetXdgSurfaceOpcode uint16 = 2
const waylandWlShmPoolCreateBufferOpcode uint16 = 0
const waylandWlSurfaceAttachOpcode uint16 = 1
const waylandXdgSurfaceGetToplevelOpcode uint16 = 1
const waylandWlSurfaceCommitOpcode uint16 = 6
const waylandWlDisplayErrorEvent uint16 = 0
const waylandFormatXrgb8888 uint32 = 1
const waylandHeaderSize uint32 = 8
const colorChannels uint32 = 4

type StateEnum int

const (
	stateNone StateEnum = iota
	stateSurfaceAckedConfigure
	stateSurfaceAttached
)

type State struct {
	wlRegistry   uint32
	wlShm        uint32
	wlShmPool    uint32 // addres of a shared resource in memory
	wlBuffer     uint32
	xdgWmBase    uint32
	xdgSurface   uint32
	wlCompositor uint32
	wlSurface    uint32
	xdgToplevel  uint32
	stride       uint32 // how many bytes in a row
	w            uint32 // width
	h            uint32 // height
	shmPoolSize  uint32 // how many bytes total
	shmFd        int    // file descriptor of a shared memory resource
	shmPoolData  *[]byte
	stateState   StateEnum
}

type WaylandHeader struct {
	objectId uint32
	opcode   uint16
	msgSize  uint16
}

func DisplayConnect() (int, error) {
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		fmt.Println("Connection error: ", err)
		return -1, nil
	}
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	path := fmt.Sprintf("%s/%s", xdgRuntimeDir, waylandDisplay)
	addr := syscall.SockaddrUnix{Name: path}
	if err := syscall.Connect(fd, &addr); err != nil {
		fmt.Println("Connection error: ", err.Error())
		return -1, err
	}
	fmt.Printf("Socket file descriptor: %d\n", fd)
	return fd, nil
}

func GetRegistry(fd int) uint32 {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, waylandDisplayObjectId)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlDisplayGetRegistryOpcode)
	msgAnnouncedSize := waylandHeaderSize + 4 // 4 for size of currentId
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgAnnouncedSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	n, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("GetRegistry error: " + err.Error())
	}
	fmt.Printf("wrote %d bytes to the socket\n", n)
	return waylandCurrentId
}

func RegistryBind(fd int, registry uint32, name uint32, ifacePadded []byte, ifaceLen uint32, version uint32) uint32 {
	// registry | opcode | msg length | name | interfaceLen | interface with padding | version | wayland current id
	// 4        | 2      | 2          | 4    | 4            | len(interface padded)  | 4       | 4
	// 20 + len(interface padded) bytes
	fmt.Printf("registry bind: ifacePadded=%s, ifaceLen=%d, ifaceLenPadded=%d\n", string(ifacePadded), ifaceLen, len(ifacePadded))
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, registry)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlRegistryBindOpcode)
	msgSize := waylandHeaderSize + 4 + 4 + uint32(len(ifacePadded)) + 4 + 4 // the 4s come from the sizes of name, ifaceLen, version and waylandCurrentId variables that are all uint32
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, name)
	msg = binary.LittleEndian.AppendUint32(msg, ifaceLen)
	msg = append(msg, ifacePadded...)
	msg = binary.LittleEndian.AppendUint32(msg, version)
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("Registry bind error: " + err.Error())
	}
	return waylandCurrentId
}

func CreateSurface(fd int, state *State) uint32 {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlCompositor)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlCompositorCreateSurfaceOpcode)
	msgSize := waylandHeaderSize + 4 // 4 for waylandCurrentId
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("CreateSurface error: " + err.Error())
	}
	return waylandCurrentId
}

func GetXdgSurface(fd int, state *State) uint32 {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgWmBase)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgWmBaseGetXdgSurfaceOpcode)
	msgSize := waylandHeaderSize + 4 + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSurface)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("GetXdgSurface error: " + err.Error())
	}
	return waylandCurrentId
}

func GetXdgSurfaceTopLevel(fd int, state *State) uint32 {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgSurfaceGetToplevelOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("GetXdgSurfaceTopLevel error: " + err.Error())
	}
	return waylandCurrentId
}

func SurfaceCommit(fd int, state *State) {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlSurfaceCommitOpcode)
	msgSize := waylandHeaderSize
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("SurfaceCommit error: " + err.Error())
	}
}

func getMsgHeader(msg []byte) WaylandHeader {
	objectId := binary.LittleEndian.Uint32(msg[:4])
	opcode := binary.LittleEndian.Uint16(msg[4:6])
	msgSize := binary.LittleEndian.Uint16(msg[6:8])
	return WaylandHeader{objectId, opcode, msgSize}
}

func SendWmBasePong(pingBytes []byte, fd int, state *State) {
	fmt.Println("SEND WM BASE PONG")
	ping := binary.LittleEndian.Uint32(pingBytes[:4])
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgWmBase)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgWmBasePongOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, ping)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("SendWmBasePong error: " + err.Error())
	}
}

func SendSurfaceAckConfigure(configureBytes []byte, fd int, state *State) {
	fmt.Println("SEND SURFACE ACK")
	configure := binary.LittleEndian.Uint32(configureBytes[:4])
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgSurfaceAckConfigureOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, configure)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("SendSurfaceAckConfigur error: " + err.Error())
		return
	}
	state.stateState = stateSurfaceAckedConfigure
}

func CreateShmPool(fd int, state *State) (uint32, error) {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlShm)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlShmCreatePoolOpcode)
	msgSize := waylandHeaderSize + 4 + 4 // header + currentId + ShmPoolSize
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	msg = binary.LittleEndian.AppendUint32(msg, state.shmPoolSize)
	// _, err := syscall.Write(fd, msg)
	fmt.Println(state.shmFd)
	fmt.Println(os.Getpid())
	oob := syscall.UnixRights(state.shmFd)
	err := syscall.Sendmsg(fd, msg, oob, nil, 0)
	if err != nil {
		fmt.Println("SendSurfaceAckConfigur error: " + err.Error())
	}
	return waylandCurrentId, nil
}

func CreateShmPoolBuffer(fd int, state *State) uint32 {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlShmPool)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlShmPoolCreateBufferOpcode)
	msgSize := waylandHeaderSize + 4 + 4 + 4 + 4 + 4 + 4 // header + currentId + offset + state.w + state.h + state.stride + format
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	offset := uint32(0)
	msg = binary.LittleEndian.AppendUint32(msg, offset)
	msg = binary.LittleEndian.AppendUint32(msg, state.w)
	msg = binary.LittleEndian.AppendUint32(msg, state.h)
	msg = binary.LittleEndian.AppendUint32(msg, state.stride)
	msg = binary.LittleEndian.AppendUint32(msg, waylandFormatXrgb8888)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("SendSurfaceAckConfigure error: " + err.Error())
	}
	return waylandCurrentId
}

func SurfaceAttach(fd int, state *State) error {
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlSurfaceAttachOpcode)
	msgSize := waylandHeaderSize + 4 + 4 + 4 // header + state.wlBuffer + x + y
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, state.wlBuffer)
	x, y := uint32(0), uint32(0)
	msg = binary.LittleEndian.AppendUint32(msg, x)
	msg = binary.LittleEndian.AppendUint32(msg, y)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		return err
	}
	return nil
}

type WaylandInterface struct {
	name             uint32
	ifaceLen         uint32
	ifaceLenPadded   uint32
	iface            []byte
	ifaceWithPadding []byte
	version          uint32
}

func getMsgInterface(msg []byte) WaylandInterface {
	name := binary.LittleEndian.Uint32(msg[8:12])
	interfaceLen := binary.LittleEndian.Uint32(msg[12:16])
	interfaceLenPadded := roundUpToMultpl4(interfaceLen)
	// fmt.Printf("name=%d, interfaceLen=%d, interfaceLenPadded=%d\n", name, interfaceLen, interfaceLenPadded)
	iface := msg[16 : 16+interfaceLen]
	ifaceWithPadding := msg[16 : 16+interfaceLenPadded]
	// fmt.Printf("interface=%s\n", iface)
	version := binary.LittleEndian.Uint32(msg[16+interfaceLenPadded : 20+interfaceLenPadded])
	// fmt.Printf("version=%d\n", version) // length of interface should be 1 less than what Wayland gives, because Wayland includes the NULL in the end
	return WaylandInterface{name: name, ifaceLen: interfaceLen, ifaceLenPadded: interfaceLenPadded, iface: iface, ifaceWithPadding: ifaceWithPadding, version: version}
}

func iterfaceNameBytes(iface string, ifaceLen uint32) []byte {
	interfaceBytes := make([]byte, roundUpToMultpl4(ifaceLen))
	for i, c := range iface {
		interfaceBytes[i] = byte(c)
	}
	return interfaceBytes
}

func roundUpToMultpl4(n uint32) uint32 {
	reminder := n % 4
	if reminder == 0 {
		return n
	}
	return n + (4 - reminder)
}
