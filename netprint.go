package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	addr         = flag.String("addr", "localhost:7702", "The address on which netprint listens.")
	tcp          = flag.Bool("tcp", false, "Accept raw TCP requests instead of HTTP.")
	udp          = flag.Bool("udp", false, "Accept raw UDP packets instead of HTTP.")
	delay        = flag.Duration("delay", 0, "How long to delay before responding (HTTP only).")
	responseCode = flag.Int("response-code", http.StatusOK, "Response code for HTTP requests.")
	responseText = flag.String("response-text", "", "Response body for HTTP requests.")
	mode         = modeHTTP
	mut          = &sync.Mutex{}
)

func fatal(args ...interface{}) {
	fmt.Println(args...)
	os.Exit(1)
}

func fatalf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	os.Exit(1)
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

	time.Sleep(*delay)

	w.WriteHeader(*responseCode)
	w.Write([]byte(*responseText))
}

func runHTTP() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHTTP)
	fmt.Println("Now accepting HTTP requests on", *addr)
	return http.ListenAndServe(*addr, mux)
}

func handleTCP(conn net.Conn) {
	fmt.Printf(">>>>> %s connected.\n", conn.RemoteAddr())
	written, err, endingNewline := copyRecordNewline(os.Stdout, conn)
	if err != nil {
		return
	}
	if written == 0 {
		fmt.Println("(No data transmitted.)")
	} else if !endingNewline {
		fmt.Println()
	}
	fmt.Printf(">>>>> %s disconnected.\n", conn.RemoteAddr())
}

func runTCP() error {
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	fmt.Println("Now accepting raw TCP requests on", *addr)
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		handleTCP(conn)
	}
}

func handleUDP(conn *net.UDPConn) {
	buf := make([]byte, 10*1024) // 10KB buffer to handle pretty damn big UDP packets
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		fmt.Printf(">>>>> Received a packet from %s\n", addr)
		if n == 0 {
			fmt.Println("(No data transmitted.)")
		} else {
			os.Stdout.Write(buf[:n])
			if buf[n-1] != '\n' {
				fmt.Println()
			}
		}
	}
}

func runUDP() error {
	u, err := net.ResolveUDPAddr("udp", *addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", u)
	if err != nil {
		return err
	}
	fmt.Println("Now accepting raw UDP requests on", *addr)
	handleUDP(conn)
	return nil
}

func main() {
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

	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	for _, f := range []string{"response-code", "response-text", "delay"} {
		if setFlags[f] && mode != modeHTTP {
			fatalf("Cannot specify -%s except in HTTP mode.", f)
		}
	}
	if setFlags["response-code"] && (*responseCode < 100 || *responseCode >= 600) {
		fatal("Invalid HTTP response code:", *responseCode)
	}
	switch mode {
	case modeHTTP:
		fatal(runHTTP())
	case modeTCP:
		fatal(runTCP())
	case modeUDP:
		fatal(runUDP())
	}
}
