package siphon

import (
	"fmt"
)


func NewAddr(label string, proto string, addr string) (siphon Addr) {
	siphon = Addr{}
	siphon.Label = label
	switch proto {
	case "unix":
	case "tcp":
	default: panic(fmt.Errorf("Unsupported protocol \"%s\"", proto))
	}
	siphon.Proto = proto
	siphon.Addr = addr
	return
}

func NewInternalAddr() (siphon Addr) {
	siphon = Addr{}
	siphon.Label = "internal"
	siphon.Proto = "internal"
	return
}

type Addr struct {
	Label     string
	Proto     string
	Addr      string
}
