package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy represents the DPI circumvention strategy
type Strategy int32

const (
	StrategySplit1     Strategy = 0 // Split at byte 1 (separates initial TLS record byte)
	StrategyMicroChunk Strategy = 1 // 3-way packet micro-fragmentation (1, 10, rest)
)

func (s Strategy) String() string {
	switch s {
	case StrategySplit1:
		return "Split-1 (Split at byte 1)"
	case StrategyMicroChunk:
		return "Micro-Chunk (3-way packet fragmentation: 1, 10, rest)"
	default:
		return "Split-1 (Default)"
	}
}

// Global engine state
var (
	currentStrategy atomic.Int32
	statsBypassed   atomic.Uint64
	statsDirect     atomic.Uint64
	bufferPool      = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 64*1024)
			return &b
		},
	}
)

func init() {
	currentStrategy.Store(int32(StrategySplit1))
}

// GetStrategy returns the active desynchronization strategy
func GetStrategy() Strategy {
	return Strategy(currentStrategy.Load())
}

// SetStrategy updates the active desynchronization strategy
func SetStrategy(s Strategy) {
	currentStrategy.Store(int32(s))
}

// CycleStrategy switches to the next available working strategy
func CycleStrategy() Strategy {
	next := (GetStrategy() + 1) % 2
	SetStrategy(next)
	return next
}

// Direct pass-through fastpath domains (LAN, localhost, and benchmark networks)
var directDomains = []string{
	"localhost", "127.0.0.1", "::1",
	"speedtest.net", "fast.com",
}

// ShouldBypassDPI determines whether TLS ClientHello fragmentation should be applied.
// By default, all outbound HTTPS connections are desynchronized to defeat DPI inspection.
func ShouldBypassDPI(host string) bool {
	h := strings.ToLower(host)
	if idx := strings.Index(h, ":"); idx != -1 {
		h = h[:idx]
	}

	// Bypass local and benchmark hosts
	for _, d := range directDomains {
		if h == d || strings.HasSuffix(h, "."+d) {
			return false
		}
	}

	// Apply DPI desynchronization to all outbound HTTPS traffic
	return true
}

// ParsedTLS holds SNI metadata extracted from TLS ClientHello
type ParsedTLS struct {
	IsClientHello bool
	SNI           string
	SNIOffset     int
	SNILength     int
}

// ParseClientHello analyzes TLS initial handshake record for SNI extension
func ParseClientHello(data []byte) ParsedTLS {
	res := ParsedTLS{}
	if len(data) < 5 {
		return res
	}

	// TLS Record: ContentType Handshake (0x16), Version (0x03, 0x01..0x03)
	if data[0] != 0x16 || data[1] != 0x03 {
		return res
	}

	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < 5+recordLen {
		return res
	}

	// Handshake type: ClientHello (0x01)
	if data[5] != 0x01 {
		return res
	}

	pos := 5 + 4 // Skip Handshake Type (1) + Length (3)
	if pos+34 > len(data) {
		return res
	}

	pos += 2 + 32 // Skip Version (2) + Random (32)

	// Session ID
	if pos >= len(data) {
		return res
	}
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen

	// Cipher Suites
	if pos+2 > len(data) {
		return res
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + cipherSuitesLen

	// Compression Methods
	if pos+1 > len(data) {
		return res
	}
	compMethodsLen := int(data[pos])
	pos += 1 + compMethodsLen

	// Extensions
	if pos+2 > len(data) {
		return res
	}
	extensionsLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2

	extensionsEnd := pos + extensionsLen
	if extensionsEnd > len(data) {
		extensionsEnd = len(data)
	}

	for pos+4 <= extensionsEnd {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if extType == 0x0000 && pos+extLen <= extensionsEnd {
			sniPos := pos
			if sniPos+2 > len(data) {
				break
			}
			serverNameListLen := int(binary.BigEndian.Uint16(data[sniPos : sniPos+2]))
			sniPos += 2
			sniListEnd := sniPos + serverNameListLen
			if sniListEnd > pos+extLen {
				sniListEnd = pos + extLen
			}

			for sniPos+3 <= sniListEnd {
				nameType := data[sniPos]
				nameLen := int(binary.BigEndian.Uint16(data[sniPos+1 : sniPos+3]))
				sniPos += 3
				if nameType == 0x00 && sniPos+nameLen <= sniListEnd {
					res.IsClientHello = true
					res.SNI = string(data[sniPos : sniPos+nameLen])
					res.SNIOffset = sniPos
					res.SNILength = nameLen
					return res
				}
				sniPos += nameLen
			}
		}
		pos += extLen
	}

	if data[5] == 0x01 {
		res.IsClientHello = true
	}
	return res
}

// SendFragmented dispatches the initial client payload using the selected desync strategy
func SendFragmented(targetConn net.Conn, payload []byte, parsed ParsedTLS, strategy Strategy) error {
	tcpConn, ok := targetConn.(*net.TCPConn)
	if ok {
		_ = tcpConn.SetNoDelay(true)
	}

	if len(payload) <= 1 {
		_, err := targetConn.Write(payload)
		return err
	}

	// For all hosts, find the root domain in payload to split right inside the blocked name
	idx := -1
	if parsed.SNI != "" {
		parts := strings.Split(parsed.SNI, ".")
		var rootDomain string
		if len(parts) >= 2 {
			rootDomain = parts[len(parts)-2] + "." + parts[len(parts)-1]
		} else {
			rootDomain = parsed.SNI
		}
		found := bytes.Index(payload, []byte(rootDomain))
		if found > 0 {
			mid := 3
			if len(rootDomain) <= 3 {
				mid = 1
			}
			if found+mid < len(payload) {
				idx = found + mid
			}
		}
	}

	if idx == -1 {
		for _, target := range []string{"roblox.com", "discord.com", "discord.gg", "discordapp.com"} {
			found := bytes.Index(payload, []byte(target))
			if found > 0 && found+3 < len(payload) {
				idx = found + 3
				break
			}
		}
	}

	if idx > 1 && idx < len(payload) {
		if err := writeSegment(targetConn, payload[:1], 3*time.Millisecond); err != nil {
			return err
		}
		if err := writeSegment(targetConn, payload[1:idx], 3*time.Millisecond); err != nil {
			return err
		}
		_, err := targetConn.Write(payload[idx:])
		return err
	}

	switch strategy {
	case StrategyMicroChunk:
		if err := writeSegment(targetConn, payload[:1], 2*time.Millisecond); err != nil {
			return err
		}
		if len(payload) > 10 {
			if err := writeSegment(targetConn, payload[1:10], 2*time.Millisecond); err != nil {
				return err
			}
			_, err := targetConn.Write(payload[10:])
			return err
		}
		_, err := targetConn.Write(payload[1:])
		return err

	case StrategySplit1:
		fallthrough
	default:
		if err := writeSegment(targetConn, payload[:1], 2*time.Millisecond); err != nil {
			return err
		}
		_, err := targetConn.Write(payload[1:])
		return err
	}
}

func writeSegment(conn net.Conn, chunk []byte, delay time.Duration) error {
	if len(chunk) == 0 {
		return nil
	}
	if _, err := conn.Write(chunk); err != nil {
		return err
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	return nil
}

// SplicePipes duplex-copies between client and server with zero memory allocation
func SplicePipes(clientReader io.Reader, clientConn, serverConn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// client -> server
	go func() {
		defer wg.Done()
		bufPtr := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bufPtr)

		buf := *bufPtr
		for {
			nr, er := clientReader.Read(buf)
			if nr > 0 {
				nw, ew := serverConn.Write(buf[0:nr])
				if ew != nil || nr != nw {
					break
				}
			}
			if er != nil {
				break
			}
		}
		_ = serverConn.Close()
		_ = clientConn.Close()
	}()

	// server -> client
	go func() {
		defer wg.Done()
		bufPtr := bufferPool.Get().(*[]byte)
		defer bufferPool.Put(bufPtr)

		buf := *bufPtr
		for {
			nr, er := serverConn.Read(buf)
			if nr > 0 {
				nw, ew := clientConn.Write(buf[0:nr])
				if ew != nil || nr != nw {
					break
				}
			}
			if er != nil {
				break
			}
		}
		_ = clientConn.Close()
		_ = serverConn.Close()
	}()

	wg.Wait()
}
