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

	robloxApiCluster = []string{
		"128.116.32.3", "128.116.2.3", "128.116.30.3", "128.116.48.3",
		"128.116.56.3", "128.116.13.3", "128.116.21.3", "128.116.44.3",
	}
	robloxWebCluster = []string{
		"128.116.13.3", "128.116.32.3", "128.116.2.3", "128.116.30.3",
		"128.116.48.3", "128.116.56.3",
	}
	robloxUsersCluster = []string{
		"128.116.32.3", "128.116.13.3", "128.116.2.3", "128.116.48.3",
		"128.116.56.3",
	}
	robloxAccountCluster = []string{
		"128.116.2.3", "128.116.48.3", "128.116.56.3", "128.116.32.3",
		"128.116.13.3",
	}
	robloxClientSettingsCluster = []string{
		"128.116.30.3", "128.116.48.3", "128.116.56.3", "128.116.32.3",
		"128.116.13.3",
	}
	robloxGameJoinCluster = []string{
		"128.116.13.3", "128.116.32.3", "128.116.48.3", "128.116.56.3",
		"128.116.21.3", "128.116.44.3", "128.116.30.3",
	}
	robloxThumbnailsCluster = []string{
		"128.116.5.3", "128.116.2.3", "128.116.21.3", "128.116.31.3",
		"128.116.13.3", "128.116.32.3",
	}
	robloxAvatarCluster = []string{
		"128.116.13.3", "128.116.32.3", "128.116.2.3", "128.116.30.3",
		"128.116.48.3", "128.116.56.3",
	}
	robloxAssetDeliveryCluster = []string{
		"128.116.48.3", "128.116.5.3", "128.116.13.3", "128.116.32.3",
		"128.116.2.3", "128.116.30.3",
	}
	robloxVersionCompatibilityCluster = []string{
		"128.116.2.3", "128.116.5.3", "128.116.13.3", "128.116.21.3",
		"128.116.31.3", "128.116.48.3", "128.116.56.3",
	}

	pinnedHosts = map[string][]string{
		// Roblox Core Web & Auth
		"roblox.com":                          robloxWebCluster,
		"www.roblox.com":                      robloxWebCluster,
		"auth.roblox.com":                     {"104.18.2.63", "104.18.3.63"},
		"accountsettings.roblox.com":          robloxAccountCluster,
		"accountinformation.roblox.com":      robloxAccountCluster,
		"users.roblox.com":                    robloxUsersCluster,
		"apis.roblox.com":                     robloxApiCluster,
		"ecsv2.roblox.com":                    robloxApiCluster,
		"realtime-signalr.roblox.com":         robloxApiCluster,
		"notifications.roblox.com":            robloxApiCluster,
		"gamejoin.roblox.com":                 robloxGameJoinCluster,
		"assetgame.roblox.com":                robloxGameJoinCluster,
		"assetdelivery.roblox.com":            robloxAssetDeliveryCluster,
		"contentdelivery.roblox.com":          robloxAssetDeliveryCluster,
		"privatemessages.roblox.com":          robloxUsersCluster,
		"contacts.roblox.com":                 robloxUsersCluster,
		"metrics.roblox.com":                  robloxApiCluster,
		"versioncompatibility.api.roblox.com": robloxVersionCompatibilityCluster,
		"clientsettings.roblox.com":            robloxClientSettingsCluster,
		"clientsettingscdn.roblox.com":         {"18.172.242.115", "18.172.242.75", "18.172.242.85", "18.172.242.109"},
		"setup.rbxcdn.com":                    {"2.16.16.154", "2.16.16.156"},
		"contentstore.roblox.com":             robloxAssetDeliveryCluster,
		"locale.roblox.com":                   robloxWebCluster,
		"voice.roblox.com":                    robloxApiCluster,
		"thumbnails.roblox.com":               robloxThumbnailsCluster,
		"avatar.roblox.com":                   robloxAvatarCluster,
		"games.roblox.com":                    robloxWebCluster,
		"presence.roblox.com":                 robloxUsersCluster,
		"friends.roblox.com":                  robloxUsersCluster,
		"chat.roblox.com":                     robloxUsersCluster,
		"groups.roblox.com":                   robloxUsersCluster,
		"economy.roblox.com":                  robloxAvatarCluster,
		"inventory.roblox.com":                robloxAvatarCluster,
		"develop.roblox.com":                  robloxAvatarCluster,
		"badges.roblox.com":                   robloxAvatarCluster,
		"ephemeralcounters.api.roblox.com":    robloxApiCluster,
		"gameinternationalization.roblox.com": robloxApiCluster,

		// Roblox Catalog, Item Configuration & Matchmaking
		"catalog.roblox.com":               robloxAvatarCluster,
		"itemconfiguration.roblox.com":     robloxAvatarCluster,
		"itemconfiguration.api.roblox.com": robloxAvatarCluster,
		"matchmaking.roblox.com":           robloxGameJoinCluster,
		"client-telemetry.roblox.com":      robloxApiCluster,
		"trades.roblox.com":                robloxUsersCluster,
		"premiumfeatures.roblox.com":       robloxWebCluster,
		"translations.roblox.com":          robloxWebCluster,
		"followings.roblox.com":            robloxUsersCluster,
		"engagement.roblox.com":            robloxApiCluster,
		"private-messages.roblox.com":      robloxUsersCluster,
		"share.roblox.com":                 robloxWebCluster,

		// Roblox 3D Avatar Rendering & Asset CDNs
		"tr.rbxcdn.com":               {"18.239.255.42", "18.239.255.127", "18.239.255.86", "18.239.255.13"},
		"images.rbxcdn.com":           {"65.9.9.117", "18.239.255.42", "18.239.255.127"},
		"static.rbxcdn.com":           {"3.169.85.61", "18.239.255.42", "18.239.255.127"},
		"js.rbxcdn.com":               {"108.157.52.100", "18.239.255.42", "18.239.255.127"},
		"css.rbxcdn.com":              {"3.160.57.45", "18.239.255.42", "18.239.255.127"},
		"c0.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c1.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c2.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c3.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c4.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c5.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c6.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"c7.rbxcdn.com":               {"2.22.89.43", "2.16.16.154", "18.239.255.42"},
		"t0.rbxcdn.com":               {"65.9.9.110", "18.239.255.42", "18.239.255.127"},
		"t1.rbxcdn.com":               {"108.157.60.98", "18.239.255.42", "18.239.255.127"},
		"t2.rbxcdn.com":               {"3.160.57.94", "18.239.255.42", "18.239.255.127"},
		"t3.rbxcdn.com":               {"65.9.9.36", "18.239.255.42", "18.239.255.127"},
		"t4.rbxcdn.com":               {"3.169.85.3", "18.239.255.42", "18.239.255.127"},
		"t5.rbxcdn.com":               {"65.9.9.125", "18.239.255.42", "18.239.255.127"},
		"t6.rbxcdn.com":               {"13.35.58.53", "18.239.255.42", "18.239.255.127"},
		"t7.rbxcdn.com":               {"65.9.9.82", "18.239.255.42", "18.239.255.127"},

		// Discord Core (Cloudflare Anycast Edges)
		"discord.com":                 {"162.159.138.232", "162.159.137.232", "162.159.136.232", "162.159.135.232"},
		"www.discord.com":             {"162.159.138.232", "162.159.137.232"},
		"canary.discord.com":          {"162.159.138.232", "162.159.137.232"},
		"ptb.discord.com":             {"162.159.138.232", "162.159.137.232"},
		"discordapp.com":              {"162.159.138.232", "162.159.137.232"},
		"www.discordapp.com":          {"162.159.138.232", "162.159.137.232"},
		"gateway.discord.gg":            {"162.159.133.234", "162.159.134.234", "162.159.135.234", "162.159.136.234"},
		"remote-auth-gateway.discord.gg": {"162.159.133.234", "162.159.134.234", "162.159.135.234", "162.159.136.234", "162.159.130.234"},
		"remote-auth.discord.gg":         {"162.159.133.234", "162.159.134.234", "162.159.135.234"},
		"qr.discord.gg":                  {"162.159.138.232", "162.159.137.232"},
		"voice-gateway.discord.gg":       {"162.159.133.234", "162.159.134.234", "162.159.135.234"},
		"app.discord.com":                {"162.159.138.232", "162.159.137.232"},
		"auth.discord.com":               {"162.159.138.232", "162.159.137.232"},
		"login.discord.com":              {"162.159.138.232", "162.159.137.232"},
		"cdn.discordapp.com":             {"162.159.130.233", "162.159.133.233", "162.159.134.233", "162.159.135.233"},
		"media.discordapp.net":        {"162.159.138.232", "162.159.137.232"},
		"images-ext-1.discordapp.net": {"162.159.138.232", "162.159.137.232"},
		"images-ext-2.discordapp.net": {"162.159.138.232", "162.159.137.232"},
		"status.discord.com":          {"162.159.138.232", "162.159.137.232"},
		"router.discordapp.net":       {"162.159.138.232", "162.159.137.232"},
		"updates.discord.com":         {"162.159.138.232", "162.159.137.232"},
		"dl.discordapp.net":           {"162.159.138.232", "162.159.137.232"},
		"discord.gg":                  {"162.159.138.232", "162.159.137.232"},
		"discord.media":               {"162.159.138.232", "162.159.137.232"},
		"support.discord.com":         {"162.159.138.232", "162.159.137.232", "162.159.128.233", "162.159.135.232", "162.159.136.232"},
		"dis.gd":                      {"162.159.138.232", "162.159.137.232"},
		"discordstatus.com":           {"162.159.138.232", "162.159.137.232"},
		"discordmerch.com":            {"162.159.138.232", "162.159.137.232"},
		"discord.co":                  {"162.159.138.232", "162.159.137.232"},
		"devforum.roblox.com":         robloxWebCluster,
		"create.roblox.com":           robloxWebCluster,
		"help.roblox.com":             robloxWebCluster,
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
	} else if strings.HasSuffix(lowerHost, ".roblox.com") || lowerHost == "roblox.com" {
		result = append(result, robloxApiCluster...)
	} else if strings.HasSuffix(lowerHost, ".rbxcdn.com") {
		result = append(result, "18.239.255.42", "18.239.255.127", "2.16.16.154")
	} else if strings.HasSuffix(lowerHost, ".discord.com") || lowerHost == "discord.com" ||
		strings.HasSuffix(lowerHost, ".discordapp.com") || lowerHost == "discordapp.com" ||
		strings.HasSuffix(lowerHost, ".discord.gg") || lowerHost == "discord.gg" ||
		strings.HasSuffix(lowerHost, ".discordapp.net") || lowerHost == "discordapp.net" ||
		strings.HasSuffix(lowerHost, ".discord.media") || lowerHost == "discord.media" ||
		strings.HasSuffix(lowerHost, ".dis.gd") || lowerHost == "dis.gd" ||
		strings.HasSuffix(lowerHost, ".discordstatus.com") || lowerHost == "discordstatus.com" ||
		strings.HasSuffix(lowerHost, ".discordmerch.com") || lowerHost == "discordmerch.com" ||
		strings.HasSuffix(lowerHost, ".discord.co") || lowerHost == "discord.co" {
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
			result = append(result, "18.239.255.42", "18.239.255.127", "2.16.16.154")
		} else if strings.Contains(lowerHost, "roblox") {
			result = append(result, robloxApiCluster...)
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
		q == "rbx.com" || strings.HasSuffix(q, ".rbx.com") ||
		q == "robloxlabs.com" || strings.HasSuffix(q, ".robloxlabs.com") ||
		q == "discord.com" || strings.HasSuffix(q, ".discord.com") ||
		q == "discordapp.com" || strings.HasSuffix(q, ".discordapp.com") ||
		q == "discord.gg" || strings.HasSuffix(q, ".discord.gg") ||
		q == "discordapp.net" || strings.HasSuffix(q, ".discordapp.net") ||
		q == "discord.media" || strings.HasSuffix(q, ".discord.media") ||
		q == "dis.gd" || strings.HasSuffix(q, ".dis.gd") ||
		q == "discordstatus.com" || strings.HasSuffix(q, ".discordstatus.com") ||
		q == "discordmerch.com" || strings.HasSuffix(q, ".discordmerch.com") ||
		q == "discord.co" || strings.HasSuffix(q, ".discord.co") {
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
