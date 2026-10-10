package main

import (
	"bytes"
	"crypto/tls"
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

func handleRawDNSQuery(conn *net.UDPConn, clientAddr *net.UDPAddr, queryData []byte) {
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
