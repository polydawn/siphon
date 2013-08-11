package siphon

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	trainwreck "log"
	"strings"
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

/*

golang's default 'log' package might as well not exist.  it's like... oh hey,
we gave you this one hard coded implementation instead of an interface, because
we heard inflexibility is cool, bro.  and then we heard about those log level
things from other languages, but we didn't like them, so we decided we'd make
our own.  and hardcode those as method names.  and btw that prefix thing we
gave you?  yeah that doesn't go where you think it does... it's actually
positioned in the output stream more like where the log level is in the
logging of anyone else you've ever heard of.  there is no actual prefixing
option for your log message.  or equivalent of the "[package name]" stuff
that every other serious logging framework has provided in case you want to
logically group messages based on what they're relevant to for some wild
reason... yeah no i'm gonna have to ask you to go ahead and repeat those
strings in every log call you make.  or we thought you might like wrapping all
your logging calls in methods that add that prefix, so we decided to make it as
easy as possible to do so and have those helper functions still look like they
implement the standard logging interface, wellllll actually no, we decided to
make that completely impossible because it's not an interface.

WHAT.



sooooo i'm making io.Writer implementations that do what I want, and calling
fmt.Fprintf with them.

*/

type logTargets struct {
	host   io.Writer
	client io.Writer
	daemon io.Writer
}

var log = logTargets{}

func init() {
	enabled := make(map[string]bool)
	for _, v := range strings.Split(os.Getenv("DEBUG"), ",") {
	    enabled[v] = true
	}

	// the sheer verbosity of this without ternary operators makes me feel
	//  like i'm writing java but without all the *joy of terseness* that I would get from *java*.
	var tmp io.Writer
	if enabled["host"] || enabled["*"] { tmp = os.Stderr } else { tmp = ioutil.Discard }
	log.host   = &logWriter{prefix: "siphon: host: ",   embarassing: trainwreck.New(tmp, "", trainwreck.Lmicroseconds)}
	if enabled["client"] || enabled["*"] { tmp = os.Stderr } else { tmp = ioutil.Discard }
	log.client = &logWriter{prefix: "siphon: client: ", embarassing: trainwreck.New(tmp, "", trainwreck.Lmicroseconds)}
	if enabled["daemon"] || enabled["*"] { tmp = os.Stderr } else { tmp = ioutil.Discard }
	log.daemon = &logWriter{prefix: "siphon: daemon: ", embarassing: trainwreck.New(tmp, "", trainwreck.Lmicroseconds)}
}

type logWriter struct {
	embarassing *trainwreck.Logger
	prefix string
}

func (log *logWriter) Write(p []byte) (n int, err error) {
	log.embarassing.Printf("%s%s", log.prefix, p)
	return len(p), nil
}


