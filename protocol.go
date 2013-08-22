package siphon

import (
	"encoding/json"
	"net"
	"sync"
)

func NewNetConn(netconn net.Conn) *Conn {
	return &Conn{
		decoder: json.NewDecoder(netconn),
		encoder: json.NewEncoder(netconn),
		closerFn: func() error {
			return netconn.Close()
		},
	}
}

type Conn struct {
	decoder     *json.Decoder
	decoderLock sync.Mutex
	encoder     *json.Encoder
	encoderLock sync.Mutex
	closerFn    func()error
}

func (conn *Conn) Decode(v interface{}) error {
	conn.decoderLock.Lock()
	defer conn.decoderLock.Unlock()
	return conn.decoder.Decode(v)
}

func (conn *Conn) Encode(v interface{}) error {
	conn.encoderLock.Lock()
	defer conn.encoderLock.Unlock()
	return conn.encoder.Encode(v)
}

func (conn *Conn) Close() error {
	return conn.closerFn()
}

type Message struct {
	Content     []byte    `json:",omitempty"`
	TtyHeight   int       `json:",omitempty"`
	TtyWidth    int       `json:",omitempty"`

	// Future: should we be detecting ansi escape codes and buffering that kind of state for new clients?
	//   So i.e. color codes in effect mid-client-attach give the client the right color,
	//   and attaching to vim starts your cursor in the right place?
}
