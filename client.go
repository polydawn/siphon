package siphon

import (
	"github.com/dotcloud/docker/term"
	"fmt"
	"io"
	"os"
	"os/signal"
	"net"
	"sync"
	"syscall"
)

func Connect(addr Addr) *Client {
	conn := connectDial(addr)
	client := NewClient(addr, conn)
	client.connect()
	return client
}

func connectDial(addr Addr) *Conn {
	fmt.Fprintf(log.client, "dialing %s\r\n", addr.Label)
	conn, err := net.Dial(addr.Proto, addr.Addr)
	if err != nil {
		panic(err)
	}
	return connectHandshake(NewNetConn(conn))
}

func connectHandshake(conn *Conn) *Conn {
	if err := conn.Encode(Hello{
		Siphon: "siphon",
		Hello: "client",
	}); err != nil {
		panic(err)
	}
	var ack HelloAck
	if err := conn.Decode(&ack); err != nil {
		panic(err)
	}
	if ack.Siphon != "siphon" {
		panic(fmt.Errorf("Encountered a non-siphon.Protocol on %s, aborting", conn.Label()))
	}

	switch ack.Hello {
		case "server":
			// excellent.  we can make a client around this.
			return conn
		case "daemon":
			// we expect a redirect message from the daemon.
			var redirect Redirect
			if err := conn.Decode(&redirect); err != nil {
				panic(err)
			}
			return connectDial(redirect.Addr)
		default: panic(fmt.Errorf("Unexpected HelloAck! \"%s\"", ack.Hello))
	}
}

func NewClient(siphon Addr, conn *Conn) (client *Client) {
	client = &Client{}
	client.siphon = siphon
	client.conn = conn
	return
}

type Client struct {
	siphon     Addr

	conn       *Conn

	/** Write characters to this and they will be serialized in Siphon's wire format and shipped to the host.  Must be Connect()'d. */
	stdin      io.WriteCloser
	/** Character output from the host has been deserialized and buffered here for your reading.  Must be Connect()'d. */
	stdout     io.ReadCloser

	/** Tty size as known to the host. */
	ttyHeight  int
	/** Tty size as known to the host. */
	ttyWidth   int

	/** Unused unless Attach() called.  Typically should be os.Stdin. */
	in         io.ReadCloser
	/** Unused unless Attach() called. */
	isTerminal bool
	/** Unused unless Attach() called. */
	terminalFd uintptr
}

func (client *Client) connect() {
	stdout, stdoutPipe := io.Pipe()
	client.stdout = stdout
	go func() {
		defer client.conn.Close()
		for {
			var m Message
			if err := client.conn.Decode(&m); err != nil {
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
		defer stdinPipe.Close()
		defer client.conn.Close()
		buf := make([]byte, 32*1024)
		for {
			nr, err := stdinPipe.Read(buf)
			if nr > 0 {
				m := Message{Content:buf[0:nr]}
				if err := client.conn.Encode(&m); err != nil {
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

func (client *Client) Stdin() io.Writer {
	return client.stdin
}

func (client *Client) Stdout() io.Reader {
	return client.stdout
}

func (client *Client) Attach(in io.ReadCloser, out io.WriteCloser) {
	client.in = in
	if file, ok := client.in.(*os.File); ok {
		client.terminalFd = file.Fd()
		client.isTerminal = term.IsTerminal(client.terminalFd)
	}

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
		io.Copy(out, client.stdout)
		fmt.Fprintf(log.client, "client output closed\r\n")
	}()

	// track.Add(1) // io.Copy will block indefinitely on 'in' regardless of if client.stdin has been closed, so we can't actually wait for this.
	go func() {
		io.Copy(client.stdin, in)
		fmt.Fprintf(log.client, "client input closed\r\n")
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
	client.conn.Encode(Message{TtyHeight: height, TtyWidth: width})
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

