package siphon

import (
	"github.com/dotcloud/docker/term"
	"github.com/kr/pty"
	"encoding/json"
	"io"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

func NewHost(cmd *exec.Cmd, siphon Addr) (host Host) {
	host = Host{}
	host.siphon = siphon
	host.cmd = cmd
	host.stdout = NewWriteBroadcaster()
	host.stdin, host.stdinPipe = io.Pipe()
	return
}

type Host struct {
	siphon    Addr
	cmd       *exec.Cmd
	stdout    *WriteBroadcaster
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
	pty       *os.File

	listener  net.Listener
}

func (host *Host) Serve() {
	if host.siphon.proto == "internal" {
		return
	}
	listener, err := net.Listen(host.siphon.proto, host.siphon.addr)
	if err != nil {
		panic(err)
	}
	host.listener = listener
	go func() {
		for host.listener != nil {
			conn, err := host.listener.Accept();
			// if err.Err == net.errClosing {
			// 	break
			// } // I can't do this because net.errClosing isn't visible to me.  Yay for go's whimsically probably-weakly typed errors.
			if err != nil {
				// Also I can't check if host.listener is closed because there's no such predicate.  Good good.  Not that that wouldn't be asking for a race condition anyway.
				if err.(*net.OpError).Err.Error() == "use of closed network connection" {
					// doing this strcmp makes me feel absolutely awful, but a closed socket is normal shutdown and not panicworthy in the slightest, and I can't for the life of me find any saner way to distinguish that.
					break
				}
				panic(err)
			}
			fmt.Fprintf(log.host, "accepted new client connection\n")
			go host.handleRemoteClient(conn)
		}
	}()
}

func (host *Host) handleRemoteClient(conn net.Conn) {
	defer conn.Close()
	var track sync.WaitGroup

	// recieve client input and resize requests
	track.Add(1)
	go func() {
		dec := json.NewDecoder(conn)
		in := host.StdinPipe()
		for {
			var m Message
			if err := dec.Decode(&m); err != nil {	//FIXME: this will happily hang out long after cmd has exited if the client fails to close.
				break
			}
			if m.Content != nil {
				if _, err := in.Write(m.Content); err != nil {
					panic(err)
				}
			} else if m.TtyHeight != 0 && m.TtyWidth != 0 {
				host.Resize(m.TtyHeight, m.TtyWidth)
				//TODO: conn.Write(json.Marshal(Message{TtyHeight:m.TtyHeight, ...}))
			}
		}
		track.Done()
	}()

	// send pty output and size changes
	track.Add(1)
	go func() {
		enc := json.NewEncoder(conn)
		out := host.StdoutPipe()
		buf := make([]byte, 32*1024)
		for {
			nr, err := out.Read(buf)
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
		conn.Close()
		track.Done()
	}()

	track.Wait()
}

func (host *Host) UnServe() {
	switch x := host.listener.(type) {
	case nil:
		return
	default:
		x.Close()
		host.listener = nil
	}
}

func (host *Host) Start() {
	pty, ptySlave, err := pty.Open()
	if err != nil {
		panic(err)
	}
	host.pty = pty
	host.cmd.Stdout = ptySlave
	host.cmd.Stderr = ptySlave

	// copy output from the pty to our broadcasters
	go func() {
		defer host.stdout.CloseWriters()
		io.Copy(host.stdout, pty)
	}()

	// copy stdin from our pipe to the pty
	host.cmd.Stdin = ptySlave
	host.cmd.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
	go func() {
		defer host.stdin.Close()
		io.Copy(pty, host.stdin)
	}()

	// rets roll
	if err := host.cmd.Start(); err != nil {
		panic(err)
	}
	ptySlave.Close()
}

func (host *Host) StdinPipe() io.WriteCloser {
	return host.stdinPipe
}

func (host *Host) StdoutPipe() io.ReadCloser {
	reader, writer := io.Pipe()
	host.stdout.AddWriter(writer)
	return reader
	// DELTA: docker wraps the reader in a NewBufReader before returning.  not sure i find this the right layer for that.
}

func (host *Host) Resize(h int, w int) {
	err := term.SetWinsize(host.pty.Fd(), &term.Winsize{Height: uint16(h), Width: uint16(w)})
	if err.(syscall.Errno) != 0 {
		panic(fmt.Errorf("siphon: host error setting terminal size: %s\n", err))
	}
}

func (host *Host) cleanup() {
	if err := host.stdin.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "siphon: cleanup on %s host failed to close stdin: %s\n", host.siphon.Label, err)
	}
	host.stdout.CloseWriters()
	if err := host.pty.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "siphon: cleanup on %s host failed to close pty: %s\n", host.siphon.Label, err)
	}
}


