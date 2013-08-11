package siphon

import (
	"github.com/dotcloud/docker/term"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"net"
	"sync"
	"syscall"
)

func NewClient(siphon Addr) (client Client) {
	client = Client{}
	client.siphon = siphon

	client.in = os.Stdin
	if file, ok := client.in.(*os.File); ok {
		client.terminalFd = file.Fd()
		client.isTerminal = term.IsTerminal(client.terminalFd)
	}

	return
}

type Client struct {
	siphon     Addr

	in         io.ReadCloser
	isTerminal bool
	terminalFd uintptr

	conn       net.Conn //TODO: try to replace this with just normal io interfaces...?  want to support single-process mode.

	stdin      io.WriteCloser
	stdout     io.ReadCloser

	ttyHeight  int
	ttyWidth   int
}

func (client *Client) Connect() {
	if client.conn != nil {
		return
	}
	client.dial()
	client.initialRead()

	stdout, stdoutPipe := io.Pipe()
	client.stdout = stdout
	go func() {
		dec := json.NewDecoder(client.conn)
		defer client.conn.Close()
		for {
			var m Message
			if err := dec.Decode(&m); err != nil {
				stdoutPipe.Close()
				if err == io.EOF {
					break
				} else {
					panic(err)
				}
			}
			if m.Content != nil {
				stdoutPipe.Write(m.Content)
			} else if m.TtyHeight != 0 && m.TtyWidth != 0 {
				client.ttyHeight = m.TtyHeight
				client.ttyWidth = m.TtyWidth
				// We don't actually do much with this information.
				// We could try to force a resize of the tty we're attached to, but from a human usability standpoint that's annoying more than not.
			}
		}
	}()

	stdinPipe, stdin := io.Pipe()
	client.stdin = stdin
	go func() {
		enc := json.NewEncoder(client.conn)
		defer stdinPipe.Close()
		defer client.conn.Close()
		buf := make([]byte, 32*1024)
		for {
			nr, err := stdinPipe.Read(buf)
			if nr > 0 {
				m := Message{Content:buf[0:nr]}
				if err := enc.Encode(&m); err != nil {
					break
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
		}
	}()
}

func (client *Client) dial() {
	fmt.Fprintf(log.client, "dialing host\r\n")
	conn, err := net.Dial(client.siphon.proto, client.siphon.addr)
	if err != nil {
		panic(err)
	}
	client.conn = conn
}

func (client *Client) initialRead() {
	//TODO read intial handshake, current terminal size, etc.
}

func (client *Client) Stdin() io.Writer {
	client.Connect()
	return client.stdin
}

func (client *Client) Stdout() io.Reader {
	client.Connect()
	return client.stdout
}

func (client *Client) Attach() {
	client.Connect()

	if !client.isTerminal {
		panic(fmt.Errorf("siphon: cannot attach, no tty"))
	}

	fmt.Fprintf(log.client, "attaching to tty\r\n")

	rawOldState, err := term.SetRawTerminal(client.terminalFd)
	if err != nil {
		panic(err)
	}
	defer term.RestoreTerminal(client.terminalFd, rawOldState)

	client.monitorTtySize()

	var track sync.WaitGroup

	track.Add(1)
	go func() {
		defer track.Done()
		io.Copy(os.Stdout, client.stdout)
		fmt.Fprintf(log.client, "client stdout closed\r\n")
	}()

	track.Add(1)
	go func() {
		defer track.Done()
		io.Copy(client.stdin, os.Stdin)
		fmt.Fprintf(log.client, "client stdin closed\r\n")
	}()

	track.Wait()
}

func (client *Client) getTtySize() (h int, w int) {
	if !client.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(client.terminalFd)
	if err.(syscall.Errno) != 0 {
		panic(fmt.Errorf("siphon: client error getting terminal size: %s\n", err))
	}
	if ws == nil {
		return 0, 0
	}
	return int(ws.Height), int(ws.Width)
}

func (client *Client) sendTtyResize() {
	if client.conn == nil {
		return
	}
	height, width := client.getTtySize()
	if height == 0 && width == 0 {
		return
	}

	fmt.Fprintf(log.client, "client sending resize request to h=%d w=%d\r\n", height, width)
	m, _ := json.Marshal(Message{TtyHeight: height, TtyWidth: width})
	client.conn.Write(m)
}

func (client *Client) monitorTtySize() {
	if !client.isTerminal {
		panic(fmt.Errorf("Impossible to monitor size on non-tty"))
	}
	client.sendTtyResize()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGWINCH)
	go func() {
		for sig := range c {
			if sig == syscall.SIGWINCH {
				client.sendTtyResize()
			}
		}
	}()
}

