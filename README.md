# virtual-kbd
It lets machines be controled by keyboards not directly plugged to them.

## Server
The server listens to incoming messages over tcp. Each message from a client contains two bytes - first byte for a scancode of a key and a second byte with 1 or 0 (1 means the key was pressed, 0 that it was released). Then the information about the key event is injected through uinput. Keep in mind that for this to work you need read/write permissions for /dev/uinput device.

## Client
The client connects to a display server's unix socket to display a simple window and to get keyboard events. It also connects to the target machine's server. All the keyboard events that happen when the window is focused are then sent to the server.

### Notes
The client uses Wayland protocol to communicate with a display server. I tried it only on my machine that's using gnome. The server was tried on a Ubuntu VM and Raspberry Pi with a Raspberry Pi OS.
