package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
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
const waylandWlSeatGetKeyboardOpcode = 1
const waylandWlKeyboardKeyEventOpcode = 3
const waylandWlKeyboardModifiersOpcode = 4
const waylandShortcutsInhibitorCreateOpcode = 1

type StateEnum int

const (
	stateNone StateEnum = iota
	stateSurfaceAckedConfigure
	stateSurfaceAttached
)

type State struct {
	wlRegistry              uint32
	wlShm                   uint32
	wlShmPool               uint32 // addres of a shared resource in memory
	wlBuffer                uint32
	xdgWmBase               uint32
	xdgSurface              uint32
	wlCompositor            uint32
	wlSurface               uint32
	xdgToplevel             uint32
	stride                  uint32 // how many bytes in a row
	w                       uint32 // width of a surface
	h                       uint32 // height of a surface
	shmPoolSize             uint32 // how many bytes total in a surface
	shmFd                   int    // file descriptor of a shared memory resource
	shmPoolData             *[]byte
	wlSeat                  uint32
	wlKeyboard              uint32
	zwpShortcutsInhibitMngr uint32
	zwpShortcutsInhibitor   uint32
	stateState              StateEnum
}

type WaylandHeader struct {
	objectId uint32
	opcode   uint16
	msgSize  uint16
}

func DisplayConnect() (int, error) {
	slog.Debug("connect to a display server")
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, errors.New("socket error: " + err.Error())
	}
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	path := fmt.Sprintf("%s/%s", xdgRuntimeDir, waylandDisplay)
	addr := syscall.SockaddrUnix{Name: path}
	if err := syscall.Connect(fd, &addr); err != nil {
		return -1, errors.New("connection error: " + err.Error())
	}
	return fd, nil
}

func GetRegistry(fd int) uint32 {
	slog.Debug("request a registry")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, waylandDisplayObjectId)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlDisplayGetRegistryOpcode)
	msgAnnouncedSize := waylandHeaderSize + 4 // 4 for size of currentId
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgAnnouncedSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		fmt.Println("GetRegistry error: " + err.Error())
	}
	return waylandCurrentId
}

type WaylandInterface struct {
	name        uint32
	len         uint32
	lenPadded   uint32
	iface       []byte
	withPadding []byte
	version     uint32
}

func RegistryBind(fd int, registry uint32, iface WaylandInterface) (uint32, error) {
	slog.Debug(fmt.Sprintf("bind to an interface %s", iface.iface))
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, registry)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlRegistryBindOpcode)
	msgSize := waylandHeaderSize + 4 + 4 + uint32(iface.lenPadded) + 4 + 4 // the 4s come from the sizes of name, ifaceLen, version and waylandCurrentId variables that are all uint32
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, iface.name)
	msg = binary.LittleEndian.AppendUint32(msg, iface.len)
	msg = append(msg, iface.withPadding...)
	msg = binary.LittleEndian.AppendUint32(msg, iface.version)
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		return 0, err
	}
	return waylandCurrentId, nil
}

func CreateSurface(fd int, state *State) uint32 {
	slog.Debug("request new surface")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlCompositor)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlCompositorCreateSurfaceOpcode)
	msgSize := waylandHeaderSize + 4 // 4 for waylandCurrentId
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("request new surface failed: " + err.Error())
	}
	return waylandCurrentId
}

func GetXdgSurface(fd int, state *State) uint32 {
	slog.Debug("request xdg surface object")
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
		slog.Error("request xdg surface failed: " + err.Error())
	}
	return waylandCurrentId
}

func GetXdgSurfaceTopLevel(fd int, state *State) uint32 {
	slog.Debug("request xdg surface top level object")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgSurfaceGetToplevelOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("GetXdgSurfaceTopLevel error: " + err.Error())
	}
	return waylandCurrentId
}

func SurfaceCommit(fd int, state *State) {
	slog.Debug("commit surface")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlSurfaceCommitOpcode)
	msgSize := waylandHeaderSize
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("commit surface failed: " + err.Error())
	}
}

func getMsgHeader(msg []byte) WaylandHeader {
	objectId := binary.LittleEndian.Uint32(msg[:4])
	opcode := binary.LittleEndian.Uint16(msg[4:6])
	msgSize := binary.LittleEndian.Uint16(msg[6:8])
	return WaylandHeader{objectId, opcode, msgSize}
}

func SendWmBasePong(data []byte, fd int, state *State) {
	slog.Debug("send pong")
	pingBytes := data[waylandHeaderSize:]
	ping := binary.LittleEndian.Uint32(pingBytes[:4])
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgWmBase)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgWmBasePongOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, ping)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("send pong failed: " + err.Error())
	}
}

func SendSurfaceAckConfigure(configureBytes []byte, fd int, state *State) {
	slog.Debug("send surface ACK configure")
	configure := binary.LittleEndian.Uint32(configureBytes[:4])
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.xdgSurface)
	msg = binary.LittleEndian.AppendUint16(msg, waylandXdgSurfaceAckConfigureOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	msg = binary.LittleEndian.AppendUint32(msg, configure)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("send surface ACK configure failed: " + err.Error())
		return
	}
	state.stateState = stateSurfaceAckedConfigure
}

func CreateShmPool(fd int, state *State) (uint32, error) {
	slog.Debug("create shm pool")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlShm)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlShmCreatePoolOpcode)
	msgSize := waylandHeaderSize + 4 + 4 // header + currentId + ShmPoolSize
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	msg = binary.LittleEndian.AppendUint32(msg, state.shmPoolSize)
	oob := syscall.UnixRights(state.shmFd)
	err := syscall.Sendmsg(fd, msg, oob, nil, 0)
	if err != nil {
		slog.Error("create shm pool: " + err.Error())
	}
	return waylandCurrentId, nil
}

func CreateShmPoolBuffer(fd int, state *State) uint32 {
	slog.Debug("create shm pool buffer")
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
		slog.Error("create shm pool buffer failed: " + err.Error())
	}
	return waylandCurrentId
}

func SurfaceAttach(fd int, state *State) error {
	slog.Debug("attach surface")
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

func CreateKeyboard(fd int, state *State) uint32 {
	slog.Debug("create keyboard")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSeat)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlSeatGetKeyboardOpcode)
	msgSize := waylandHeaderSize + 4
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("create keyboard failed: " + err.Error())
	}
	return waylandCurrentId
}

// request from zwpShortcutsInhibitMngr an inhibitor object
func InhibitGlobalShortcuts(fd int, state *State) uint32 {
	slog.Debug("requesting global shortcuts inhibitor object")
	msg := make([]byte, 0)
	msg = binary.LittleEndian.AppendUint32(msg, state.zwpShortcutsInhibitMngr)
	msg = binary.LittleEndian.AppendUint16(msg, waylandWlSeatGetKeyboardOpcode)
	msgSize := waylandHeaderSize + 12
	msg = binary.LittleEndian.AppendUint16(msg, uint16(msgSize))
	waylandCurrentId++
	msg = binary.LittleEndian.AppendUint32(msg, waylandCurrentId)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSurface)
	msg = binary.LittleEndian.AppendUint32(msg, state.wlSeat)
	_, err := syscall.Write(fd, msg)
	if err != nil {
		slog.Error("requesting a global shortcuts inhibitor failed: " + err.Error())
	}
	return waylandCurrentId
}

type KeyEvent struct {
	scanCode uint32
	state    bool
}

type KeyModifiers struct {
	modsDepressed uint32
	modsLatched   uint32
	modsLocked    uint32
	group         uint32
}

func DecodeKeyEvent(data []byte) (KeyEvent, error) {
	if len(data) != 16 {
		return KeyEvent{}, errors.New(fmt.Sprintf("couldn't decode key event. data=%v", data))
	}
	ke := KeyEvent{}
	ke.scanCode = binary.LittleEndian.Uint32(data[8:12])
	state := binary.LittleEndian.Uint32(data[12:16])
	if state == 0 {
		ke.state = false
	} else {
		ke.state = true
	}
	return ke, nil
}

func DecodeKeyboardModifiersEvent(data []byte) (KeyModifiers, error) {
	km := KeyModifiers{}
	if len(data) != 20 {
		return km, errors.New(fmt.Sprintf("couldn't decode key event. data=%v", data))
	}
	km.modsDepressed = binary.LittleEndian.Uint32(data[4:8])
	km.modsLatched = binary.LittleEndian.Uint32(data[8:12])
	km.modsLocked = binary.LittleEndian.Uint32(data[12:16])
	km.group = binary.LittleEndian.Uint32(data[16:20])
	return km, nil
}

func getMsgInterface(msg []byte) WaylandInterface {
	name := binary.LittleEndian.Uint32(msg[8:12])
	interfaceLen := binary.LittleEndian.Uint32(msg[12:16])
	interfaceLenPadded := roundUpToMultpl4(interfaceLen)
	iface := msg[16 : 16+interfaceLen]
	ifaceWithPadding := msg[16 : 16+interfaceLenPadded]
	version := binary.LittleEndian.Uint32(msg[16+interfaceLenPadded : 20+interfaceLenPadded])
	return WaylandInterface{name: name, len: interfaceLen, lenPadded: interfaceLenPadded, iface: iface, withPadding: ifaceWithPadding, version: version}
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
