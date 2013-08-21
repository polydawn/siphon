package siphon

import (
	"fmt"
)


func NewAddr(label string, proto string, addr string) (siphon Addr) {
	siphon = Addr{}
	siphon.label = label
	switch proto {
	case "unix":
	case "tcp":
	default: panic(fmt.Errorf("Unsupported protocol \"%s\"", proto))
	}
	siphon.proto = proto
	siphon.addr = addr
	return
}

func NewInternalAddr() (siphon Addr) {
	siphon = Addr{}
	siphon.label = "internal"
	siphon.proto = "internal"
	return
}

type Addr struct {
	label     string
	proto     string
	addr      string
}

func (this *Addr) Label() string {
	return this.label
}
