package main

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type dohAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	Data string `json:"data"`
}

type dohResponse struct {
	Status int         `json:"Status"`
	Answer []dohAnswer `json:"Answer"`
}

type cachedDNS struct {
	ips       []string
	expiresAt time.Time
}

var (
	dnsCache   sync.Map
	dohClient  *http.Client
	dohServers = []string{
		"https://1.1.1.1/dns-query",
		"https://1.0.0.1/dns-query",
		"https://8.8.8.8/resolve",
	}

	pinnedRobloxHosts = map[string][]string{
		"roblox.com":                          {"128.116.5.3"},
		"www.roblox.com":                      {"128.116.5.3"},
		"apis.roblox.com":                     {"128.116.5.3"},
		"auth.roblox.com":                     {"104.18.3.63", "104.18.2.63"},
		"users.roblox.com":                    {"128.116.5.3"},
		"ecsv2.roblox.com":                    {"128.116.5.3"},
		"realtime-signalr.roblox.com":         {"128.116.5.3"},
		"notifications.roblox.com":            {"128.116.5.3"},
		"gamejoin.roblox.com":                 {"128.116.5.3"},
		"assetgame.roblox.com":                {"128.116.5.3"},
		"assetdelivery.roblox.com":            {"128.116.5.3"},
		"contentdelivery.roblox.com":          {"128.116.5.3"},
		"privatemessages.roblox.com":          {"128.116.5.3"},
		"contacts.roblox.com":                 {"128.116.5.3"},
		"metrics.roblox.com":                  {"128.116.5.3"},
		"versioncompatibility.api.roblox.com": {"128.116.5.3"},
		"clientsettings.roblox.com":            {"128.116.5.3"},
		"clientsettingscdn.roblox.com":         {"99.84.91.31", "18.172.242.85", "18.172.242.109"},
		"setup.rbxcdn.com":                    {"2.16.16.154", "2.16.16.156"},
		"contentstore.roblox.com":             {"128.116.5.3"},
		"locale.roblox.com":                   {"128.116.5.3"},
		"voice.roblox.com":                    {"128.116.5.3"},
		"thumbnails.roblox.com":               {"128.116.5.3"},
		"avatar.roblox.com":                   {"128.116.5.3"},
		"games.roblox.com":                    {"128.116.5.3"},
		"presence.roblox.com":                 {"128.116.5.3"},
		"friends.roblox.com":                  {"128.116.5.3"},
		"chat.roblox.com":                     {"128.116.5.3"},
		"groups.roblox.com":                   {"128.116.5.3"},
		"economy.roblox.com":                  {"128.116.5.3"},
		"inventory.roblox.com":                {"128.116.5.3"},
		"develop.roblox.com":                  {"128.116.5.3"},
		"badges.roblox.com":                   {"128.116.5.3"},
		"accountsettings.roblox.com":          {"128.116.5.3"},
		"ephemeralcounters.api.roblox.com":    {"128.116.5.3"},
		"gameinternationalization.roblox.com": {"128.116.5.3"},
	}
)

func init() {
	dohClient = &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			DisableKeepAlives: false,
		},
	}
}

// ResolveAllSecure resolves a domain and returns all working IP addresses
func ResolveAllSecure(host string) ([]string, error) {
	if net.ParseIP(host) != nil {
		return []string{host}, nil
	}

	now := time.Now()
	if val, ok := dnsCache.Load(host); ok {
		cached := val.(cachedDNS)
		if now.Before(cached.expiresAt) {
			return cached.ips, nil
		}
	}

	var validIPs []string

	for _, endpoint := range dohServers {
		url := fmt.Sprintf("%s?name=%s&type=A", endpoint, host)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/dns-json")

		resp, err := dohClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var res dohResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			continue
		}

		for _, ans := range res.Answer {
			if ans.Type == 1 {
				ip := strings.TrimSpace(ans.Data)
				// Filter known ISP sinkholes or block page IPs (e.g., 195.175.x.x)
				if strings.HasPrefix(ip, "195.175.") {
					continue
				}
				if net.ParseIP(ip) != nil {
					validIPs = append(validIPs, ip)
				}
			}
		}
		if len(validIPs) > 0 {
			break
		}
	}

	if len(validIPs) == 0 {
		ips, err := net.LookupHost(host)
		if err == nil {
			for _, ip := range ips {
				if !strings.HasPrefix(ip, "195.175.") {
					validIPs = append(validIPs, ip)
				}
			}
		}
	}

	if len(validIPs) > 0 {
		dnsCache.Store(host, cachedDNS{
			ips:       validIPs,
			expiresAt: now.Add(5 * time.Minute),
		})
		return validIPs, nil
	}

	return nil, fmt.Errorf("could not resolve %s securely", host)
}

// DialTarget dials the best accessible IP address for a host
func DialTarget(host, port string) (net.Conn, error) {
	lowerHost := strings.ToLower(host)
	if pinned, ok := pinnedRobloxHosts[lowerHost]; ok && len(pinned) > 0 {
		for _, ip := range pinned {
			addr := net.JoinHostPort(ip, port)
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err == nil {
				return conn, nil
			}
		}
	}

	ips, err := ResolveAllSecure(host)
	if err != nil {
		return net.DialTimeout("tcp", net.JoinHostPort(host, port), 4*time.Second)
	}

	var lastErr error
	for _, ip := range ips {
		addr := net.JoinHostPort(ip, port)
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// extractEmbeddedIP parses direct IP encoded in Roblox dynamic hostnames (e.g., lga2-128-116-32-3.roblox.com)
func extractEmbeddedIP(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) > 0 {
		sub := parts[0]
		dashIdx := strings.Index(sub, "-")
		if dashIdx != -1 {
			possibleIP := strings.ReplaceAll(sub[dashIdx+1:], "-", ".")
			if net.ParseIP(possibleIP) != nil {
				return possibleIP
			}
		}
	}
	return ""
}

// GetCandidateIPs returns prioritized unblocked IP addresses for a host
func GetCandidateIPs(host string) []string {
	lowerHost := strings.ToLower(host)
	var result []string

	if emb := extractEmbeddedIP(lowerHost); emb != "" {
		result = append(result, emb)
	}

	if pinned, ok := pinnedRobloxHosts[lowerHost]; ok && len(pinned) > 0 {
		result = append(result, pinned...)
	} else if strings.HasSuffix(lowerHost, ".roblox.com") {
		result = append(result, "128.116.5.3", "128.116.31.3")
	}

	ips, err := ResolveAllSecure(host)
	if err == nil {
		for _, ip := range ips {
			found := false
			for _, r := range result {
				if r == ip {
					found = true
					break
				}
			}
			if !found {
				result = append(result, ip)
			}
		}
	}
	if len(result) == 0 {
		result = append(result, host)
	}
	return result
}

// StartUDP53Listener launches an RFC 8484 DoH relay on 127.0.0.1:53
func StartUDP53Listener() error {
	addr := "127.0.0.1:53"
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	go func() {
		defer conn.Close()
		buf := make([]byte, 2048)
		for {
			n, clientAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			packet := make([]byte, n)
			copy(packet, buf[:n])
			go handleRawDNSQuery(conn, clientAddr, packet)
		}
	}()
	return nil
}

func parseDNSQueryName(data []byte) (string, int, uint16, bool) {
	if len(data) < 12 {
		return "", 0, 0, false
	}
	pos := 12
	var parts []string
	for pos < len(data) {
		length := int(data[pos])
		if length == 0 {
			pos++
			break
		}
		pos++
		if pos+length > len(data) {
			return "", 0, 0, false
		}
		parts = append(parts, string(data[pos:pos+length]))
		pos += length
	}
	if pos+4 > len(data) {
		return "", 0, 0, false
	}
	qType := binary.BigEndian.Uint16(data[pos : pos+2])
	pos += 4
	return strings.ToLower(strings.Join(parts, ".")), pos, qType, true
}

func buildLocalAResponse(queryData []byte, qEnd int) []byte {
	buf := new(bytes.Buffer)
	buf.Write(queryData[0:2])
	buf.Write([]byte{0x81, 0x80, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	buf.Write(queryData[12:qEnd])
	buf.Write([]byte{0xc0, 0x0c, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x3c, 0x00, 0x04, 127, 0, 0, 1})
	return buf.Bytes()
}

func buildLocalNODATAResponse(queryData []byte, qEnd int) []byte {
	buf := new(bytes.Buffer)
	buf.Write(queryData[0:2])
	buf.Write([]byte{0x81, 0x80, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.Write(queryData[12:qEnd])
	return buf.Bytes()
}

func handleRawDNSQuery(conn *net.UDPConn, clientAddr *net.UDPAddr, queryData []byte) {
	if qName, qEnd, qType, ok := parseDNSQueryName(queryData); ok {
		if qName == "roblox.com" || strings.HasSuffix(qName, ".roblox.com") {
			if qType == 1 { // IPv4 A record query
				resp := buildLocalAResponse(queryData, qEnd)
				_, _ = conn.WriteToUDP(resp, clientAddr)
				return
			} else if qType == 28 { // IPv6 AAAA record query - return NODATA so Windows immediately falls back to IPv4
				resp := buildLocalNODATAResponse(queryData, qEnd)
				_, _ = conn.WriteToUDP(resp, clientAddr)
				return
			}
		}
	}

	req, err := http.NewRequest(http.MethodPost, "https://1.1.1.1/dns-query", bytes.NewReader(queryData))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := dohClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	respData, err := io.ReadAll(resp.Body)
	if err != nil || len(respData) == 0 {
		return
	}

	_, _ = conn.WriteToUDP(respData, clientAddr)
}
