package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProxyServer handles dual HTTP CONNECT and SOCKS5 proxy connections
type ProxyServer struct {
	listenAddr string
	listener   net.Listener
}

// NewProxyServer creates a proxy server instance
func NewProxyServer(addr string) *ProxyServer {
	return &ProxyServer{listenAddr: addr}
}

// Start begins listening and serving connections
func (s *ProxyServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(conn)
	}
}

// Close stops the listener
func (s *ProxyServer) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *ProxyServer) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	br := bufio.NewReader(clientConn)
	firstByte, err := br.Peek(1)
	if err != nil {
		return
	}

	if firstByte[0] == 0x05 {
		s.handleSOCKS5(clientConn, br)
	} else {
		s.handleHTTPConnect(clientConn, br)
	}
}

func (s *ProxyServer) handleHTTPConnect(clientConn net.Conn, br *bufio.Reader) {
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Method != http.MethodConnect {
		s.handlePlainHTTP(clientConn, br, req)
		return
	}

	rawHost := req.Host
	host := rawHost
	port := "443"
	if strings.Contains(rawHost, ":") {
		h, p, err := net.SplitHostPort(rawHost)
		if err == nil {
			host = h
			port = p
		}
	}

	targetConn, err := DialTarget(host, port)
	if err != nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	// Read initial payload from client
	initialBuf := make([]byte, 4096)
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := br.Read(initialBuf)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil || n == 0 {
		return
	}

	// Ensure we read the complete TLS record if it's a handshake
	if n >= 5 && initialBuf[0] == 0x16 {
		recordLen := int(initialBuf[3])<<8 | int(initialBuf[4])
		totalNeeded := 5 + recordLen
		for n < totalNeeded && n < len(initialBuf) {
			nr, er := br.Read(initialBuf[n:])
			if er != nil {
				break
			}
			n += nr
		}
	}

	payload := initialBuf[:n]
	parsed := ParseClientHello(payload)

	strat := GetStrategy()
	targetHost := host
	if parsed.SNI != "" {
		targetHost = parsed.SNI
	}

	if parsed.IsClientHello && ShouldBypassDPI(targetHost) {
		statsBypassed.Add(1)
		LogBypass(targetHost, strat.String())
		if err := SendFragmented(targetConn, payload, parsed, strat); err != nil {
			return
		}
	} else {
		statsDirect.Add(1)
		LogDirect(targetHost)
		if _, err := targetConn.Write(payload); err != nil {
			return
		}
	}

	SplicePipes(br, clientConn, targetConn)
}

func (s *ProxyServer) handlePlainHTTP(clientConn net.Conn, br *bufio.Reader, req *http.Request) {
	rawHost := req.Host
	host := rawHost
	port := "80"
	if strings.Contains(rawHost, ":") {
		h, p, err := net.SplitHostPort(rawHost)
		if err == nil {
			host = h
			port = p
		}
	}

	targetConn, err := DialTarget(host, port)
	if err != nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer targetConn.Close()

	statsDirect.Add(1)
	LogDirect(host)

	if err := req.Write(targetConn); err != nil {
		return
	}
	SplicePipes(br, clientConn, targetConn)
}

func (s *ProxyServer) handleSOCKS5(clientConn net.Conn, br *bufio.Reader) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(br, header); err != nil {
		return
	}
	numMethods := int(header[1])
	methods := make([]byte, numMethods)
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}

	if _, err := clientConn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(br, reqHeader); err != nil {
		return
	}

	if reqHeader[1] != 0x01 {
		_, _ = clientConn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var targetHost string
	switch reqHeader[3] {
	case 0x01: // IPv4
		ipBuf := make([]byte, 4)
		if _, err := io.ReadFull(br, ipBuf); err != nil {
			return
		}
		targetHost = net.IP(ipBuf).String()
	case 0x03: // Domain name
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(br, lenBuf); err != nil {
			return
		}
		domainBuf := make([]byte, int(lenBuf[0]))
		if _, err := io.ReadFull(br, domainBuf); err != nil {
			return
		}
		targetHost = string(domainBuf)
	case 0x04: // IPv6
		ipBuf := make([]byte, 16)
		if _, err := io.ReadFull(br, ipBuf); err != nil {
			return
		}
		targetHost = net.IP(ipBuf).String()
	default:
		return
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(br, portBuf); err != nil {
		return
	}
	targetPort := int(portBuf[0])<<8 | int(portBuf[1])

	targetConn, err := DialTarget(targetHost, strconv.Itoa(targetPort))
	if err != nil {
		_, _ = clientConn.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer targetConn.Close()

	if _, err := clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	initialBuf := make([]byte, 4096)
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := br.Read(initialBuf)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil || n == 0 {
		return
	}

	if n >= 5 && initialBuf[0] == 0x16 {
		recordLen := int(initialBuf[3])<<8 | int(initialBuf[4])
		totalNeeded := 5 + recordLen
		for n < totalNeeded && n < len(initialBuf) {
			nr, er := br.Read(initialBuf[n:])
			if er != nil {
				break
			}
			n += nr
		}
	}

	payload := initialBuf[:n]
	parsed := ParseClientHello(payload)

	strat := GetStrategy()
	targetDisplay := targetHost
	if parsed.SNI != "" {
		targetDisplay = parsed.SNI
	}

	if parsed.IsClientHello && ShouldBypassDPI(targetDisplay) {
		statsBypassed.Add(1)
		LogBypass(targetDisplay, strat.String())
		if err := SendFragmented(targetConn, payload, parsed, strat); err != nil {
			return
		}
	} else {
		statsDirect.Add(1)
		LogDirect(targetDisplay)
		if _, err := targetConn.Write(payload); err != nil {
			return
		}
	}

	SplicePipes(br, clientConn, targetConn)
}
