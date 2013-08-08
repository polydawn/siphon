package siphon

type Message struct {
	Content     []byte    `json:",omitempty"`
	TtyHeight   int       `json:",omitempty"`
	TtyWidth    int       `json:",omitempty"`

	// Future: should we be detecting ansi escape codes and buffering that kind of state for new clients?
	//   So i.e. color codes in effect mid-client-attach give the client the right color,
	//   and attaching to vim starts your cursor in the right place?
}
