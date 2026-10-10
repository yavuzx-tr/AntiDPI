package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TransparentRelay provides kernel-free userspace SNI routing on 127.0.0.1:443 and HTTP on 127.0.0.1:80
type TransparentRelay struct {
	httpsListener net.Listener
	httpListener  net.Listener
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// StartTransparentRelay initializes transparent listeners on 127.0.0.1:443 and 127.0.0.1:80
func StartTransparentRelay() (*TransparentRelay, error) {
	httpsL, err := net.Listen("tcp", "127.0.0.1:443")
	if err != nil {
		return nil, fmt.Errorf("failed to bind 127.0.0.1:443: %w", err)
	}

	httpL, err := net.Listen("tcp", "127.0.0.1:80")
	if err != nil {
		httpsL.Close()
		return nil, fmt.Errorf("failed to bind 127.0.0.1:80: %w", err)
	}

	r := &TransparentRelay{
		httpsListener: httpsL,
		httpListener:  httpL,
		stopChan:      make(chan struct{}),
	}

	r.wg.Add(2)
	go r.serveHTTPS()
	go r.serveHTTP()

	return r, nil
}

// Close terminates transparent listeners
func (r *TransparentRelay) Close() {
	close(r.stopChan)
	if r.httpsListener != nil {
		_ = r.httpsListener.Close()
	}
	if r.httpListener != nil {
		_ = r.httpListener.Close()
	}
	r.wg.Wait()
}

func (r *TransparentRelay) serveHTTPS() {
	defer r.wg.Done()
	for {
		conn, err := r.httpsListener.Accept()
		if err != nil {
			select {
			case <-r.stopChan:
				return
			default:
				return
			}
		}
		go r.handleHTTPSConn(conn)
	}
}

func (r *TransparentRelay) handleHTTPSConn(clientConn net.Conn) {
	defer clientConn.Close()

	if tc, ok := clientConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	// Read initial TLS handshake record
	initialBuf := make([]byte, 16384)
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := clientConn.Read(initialBuf)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil || n == 0 {
		return
	}

	// Complete TLS record if fragmented across TCP
	if n >= 5 && initialBuf[0] == 0x16 {
		recordLen := int(initialBuf[3])<<8 | int(initialBuf[4])
		totalNeeded := 5 + recordLen
		for n < totalNeeded && n < len(initialBuf) {
			_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			nr, er := clientConn.Read(initialBuf[n:])
			_ = clientConn.SetReadDeadline(time.Time{})
			if er != nil {
				break
			}
			n += nr
		}
	}

	payload := initialBuf[:n]
	parsed := ParseClientHello(payload)

	targetHost := parsed.SNI
	if targetHost == "" {
		targetHost = "roblox.com"
	}

	candidateIPs := GetCandidateIPs(targetHost)
	strat := GetStrategy()

	var targetConn net.Conn

	for _, ip := range candidateIPs {
		addr := net.JoinHostPort(ip, "443")
		conn, err := net.DialTimeout("tcp", addr, 2500*time.Millisecond)
		if err != nil {
			continue
		}

		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}

		if err := SendFragmented(conn, payload, parsed, strat); err != nil {
			conn.Close()
			continue
		}

		// Verify server response (TLS ServerHello) before committing
		_ = conn.SetReadDeadline(time.Now().Add(2500 * time.Millisecond))
		respBuf := make([]byte, 16384)
		nr, err := conn.Read(respBuf)
		_ = conn.SetReadDeadline(time.Time{})

		if err != nil || nr == 0 {
			conn.Close()
			continue
		}

		// Forward ServerHello to client
		if _, err := clientConn.Write(respBuf[:nr]); err != nil {
			conn.Close()
			return
		}

		targetConn = conn
		statsBypassed.Add(1)
		LogBypass(targetHost, strat.String())
		break
	}

	if targetConn == nil {
		return
	}
	defer targetConn.Close()

	// Splice raw bidirectional TCP streams
	SplicePipes(clientConn, clientConn, targetConn)
}

func (r *TransparentRelay) serveHTTP() {
	defer r.wg.Done()
	for {
		conn, err := r.httpListener.Accept()
		if err != nil {
			select {
			case <-r.stopChan:
				return
			default:
				return
			}
		}
		go r.handleHTTPConn(conn)
	}
}

func (r *TransparentRelay) handleHTTPConn(clientConn net.Conn) {
	defer clientConn.Close()

	if tc, ok := clientConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	br := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	host := req.Host
	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err == nil {
			host = h
		}
	}

	targetConn, err := DialTarget(host, "80")
	if err != nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	if tc, ok := targetConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	req.RequestURI = ""
	req.Header.Del("Proxy-Connection")

	if err := req.Write(targetConn); err != nil {
		return
	}

	statsDirect.Add(1)
	LogDirect(host)
	SplicePipes(br, clientConn, targetConn)
}
