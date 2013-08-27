package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

var (
	addr = flag.String("addr", "localhost:7702", "The address on which netprint listens.")
	tcp  = flag.Bool("tcp", false, "Accept raw TCP requests instead of HTTP.")
	udp  = flag.Bool("udp", false, "Accept raw UDP packets instead of HTTP.")
	mode = modeHTTP
	mut  sync.Mutex
)

func fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}

func init() {
	flag.Parse()
	if *tcp {
		if *udp {
			fatal("Cannot specify both -tcp and -udp.")
		}
		mode = modeTCP
	}
	if *udp {
		mode = modeUDP
	}
}

type modeType int

const (
	modeHTTP modeType = iota
	modeTCP
	modeUDP
)

// copyRecordNewline is like io.Copy, but says whether the copied data ended in a newline.
func copyRecordNewline(dst io.Writer, src io.Reader) (written int64, err error, endingNewline bool) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
			endingNewline = buf[nr-1] == '\n'
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err, endingNewline
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	// It's pretty dumb to synchronize all the HTTP handlers when net/http is doing such a good job of
	// multiplexing requests onto goroutines for me, but this approach is simpler than constructing a
	// non-concurrent HTTP server.
	mut.Lock()
	defer mut.Unlock()

	fmt.Printf(">>>>> Request: %s\n", r.URL)
	written, err, endingNewline := copyRecordNewline(os.Stdout, r.Body)
	if err != nil {
		return
	}
	if written == 0 {
		fmt.Println("(Empty body.)")
	} else if !endingNewline {
		fmt.Println()
	}
}

func runHTTP() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHTTP)
	fmt.Println("Now listening on", *addr)
	return http.ListenAndServe(*addr, mux)
}

func main() {
	switch mode {
	case modeHTTP:
		fatal(runHTTP())
	case modeTCP:
		fatal("TCP not implemented.")
	case modeUDP:
		fatal("UDP not implemented")
	}
}
