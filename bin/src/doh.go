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

	pinnedHosts = map[string][]string{
		// Roblox Core
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

		// Roblox Catalog, Item Configuration & Matchmaking
		"catalog.roblox.com":                  {"128.116.5.3"},
		"itemconfiguration.roblox.com":        {"128.116.5.3"},
		"itemconfiguration.api.roblox.com":    {"128.116.5.3"},
		"matchmaking.roblox.com":              {"128.116.5.3"},
		"client-telemetry.roblox.com":         {"128.116.5.3"},
		"trades.roblox.com":                   {"128.116.5.3"},
		"premiumfeatures.roblox.com":          {"128.116.5.3"},
		"translations.roblox.com":             {"128.116.5.3"},
		"followings.roblox.com":               {"128.116.5.3"},
		"engagement.roblox.com":               {"128.116.5.3"},
		"private-messages.roblox.com":         {"128.116.5.3"},
		"share.roblox.com":                    {"128.116.5.3"},

		// Roblox 3D Avatar Rendering & Asset CDNs
		"tr.rbxcdn.com":                       {"18.239.255.42", "18.239.255.127", "18.239.255.86", "18.239.255.13"},
		"images.rbxcdn.com":                   {"65.9.9.117", "18.239.255.42"},
		"static.rbxcdn.com":                   {"3.169.85.61", "18.239.255.42"},
		"js.rbxcdn.com":                       {"108.157.52.100", "18.239.255.42"},
		"css.rbxcdn.com":                      {"3.160.57.45", "18.239.255.42"},
		"c0.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c1.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c2.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c3.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c4.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c5.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c6.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"c7.rbxcdn.com":                       {"2.22.89.43", "2.16.16.154"},
		"t0.rbxcdn.com":                       {"65.9.9.110", "18.239.255.42"},
		"t1.rbxcdn.com":                       {"108.157.60.98", "18.239.255.42"},
		"t2.rbxcdn.com":                       {"3.160.57.94", "18.239.255.42"},
		"t3.rbxcdn.com":                       {"65.9.9.36", "18.239.255.42"},
		"t4.rbxcdn.com":                       {"3.169.85.3", "18.239.255.42"},
		"t5.rbxcdn.com":                       {"65.9.9.125", "18.239.255.42"},
		"t6.rbxcdn.com":                       {"13.35.58.53", "18.239.255.42"},
		"t7.rbxcdn.com":                       {"65.9.9.82", "18.239.255.42"},

		// Discord Core (Cloudflare Anycast Edges)
		"discord.com":                         {"162.159.138.232", "162.159.137.232", "162.159.136.232", "162.159.135.232"},
		"www.discord.com":                     {"162.159.138.232", "162.159.137.232"},
		"canary.discord.com":                  {"162.159.138.232", "162.159.137.232"},
		"ptb.discord.com":                     {"162.159.138.232", "162.159.137.232"},
		"discordapp.com":                      {"162.159.138.232", "162.159.137.232"},
		"www.discordapp.com":                  {"162.159.138.232", "162.159.137.232"},
		"gateway.discord.gg":                  {"162.159.133.234", "162.159.134.234", "162.159.135.234", "162.159.136.234"},
		"cdn.discordapp.com":                  {"162.159.130.233", "162.159.133.233", "162.159.134.233", "162.159.135.233"},
		"media.discordapp.net":                {"162.159.138.232", "162.159.137.232"},
		"images-ext-1.discordapp.net":         {"162.159.138.232", "162.159.137.232"},
		"images-ext-2.discordapp.net":         {"162.159.138.232", "162.159.137.232"},
		"status.discord.com":                  {"162.159.138.232", "162.159.137.232"},
		"router.discordapp.net":               {"162.159.138.232", "162.159.137.232"},
		"updates.discord.com":                 {"162.159.138.232", "162.159.137.232"},
		"dl.discordapp.net":                   {"162.159.138.232", "162.159.137.232"},
		"discord.gg":                          {"162.159.138.232", "162.159.137.232"},
		"discord.media":                       {"162.159.138.232", "162.159.137.232"},
	}
	pinnedRobloxHosts = pinnedHosts
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

	if pinned, ok := pinnedHosts[lowerHost]; ok && len(pinned) > 0 {
		result = append(result, pinned...)
	} else if strings.HasSuffix(lowerHost, ".roblox.com") {
		result = append(result, "128.116.5.3")
	} else if strings.HasSuffix(lowerHost, ".rbxcdn.com") {
		result = append(result, "18.239.255.42", "2.16.16.154")
	} else if strings.HasSuffix(lowerHost, ".discord.com") ||
		strings.HasSuffix(lowerHost, ".discordapp.com") ||
		strings.HasSuffix(lowerHost, ".discord.gg") ||
		strings.HasSuffix(lowerHost, ".discordapp.net") ||
		strings.HasSuffix(lowerHost, ".discord.media") {
		result = append(result, "162.159.138.232", "162.159.137.232")
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

	// Filter out loopback IPs and ISP sinkholes so relay never dials itself or blocked page
	var filtered []string
	for _, ip := range result {
		if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" || strings.HasPrefix(ip, "195.175.") {
			continue
		}
		filtered = append(filtered, ip)
	}
	result = filtered

	if len(result) == 0 {
		if strings.Contains(lowerHost, "discord") {
			result = append(result, "162.159.138.232")
		} else if strings.Contains(lowerHost, "rbxcdn") {
			result = append(result, "18.239.255.42", "2.16.16.154")
		} else if strings.Contains(lowerHost, "roblox") {
			result = append(result, "128.116.5.3")
		} else {
			result = append(result, host)
		}
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

func isLocalBypassDomain(qName string) bool {
	q := strings.ToLower(strings.TrimSuffix(qName, "."))
	if q == "roblox.com" || strings.HasSuffix(q, ".roblox.com") ||
		q == "rbxcdn.com" || strings.HasSuffix(q, ".rbxcdn.com") ||
		q == "discord.com" || strings.HasSuffix(q, ".discord.com") ||
		q == "discordapp.com" || strings.HasSuffix(q, ".discordapp.com") ||
		q == "discord.gg" || strings.HasSuffix(q, ".discord.gg") ||
		q == "discordapp.net" || strings.HasSuffix(q, ".discordapp.net") ||
		q == "discord.media" || strings.HasSuffix(q, ".discord.media") {
		return true
	}
	return false
}

func handleRawDNSQuery(conn *net.UDPConn, clientAddr *net.UDPAddr, queryData []byte) {
	if qName, qEnd, qType, ok := parseDNSQueryName(queryData); ok {
		if isLocalBypassDomain(qName) {
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

	for _, endpoint := range dohServers {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(queryData))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")

		resp, err := dohClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		respData, err := io.ReadAll(resp.Body)
		if err != nil || len(respData) == 0 {
			continue
		}

		_, _ = conn.WriteToUDP(respData, clientAddr)
		return
	}
}
