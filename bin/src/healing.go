package main

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"
)

// StartSelfHealingEngine runs background probes and dynamic strategy optimization silently
func StartSelfHealingEngine(proxyAddr string) {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()

	time.AfterFunc(10*time.Second, func() {
		probeEndpoints(proxyAddr)
	})

	for range ticker.C {
		probeEndpoints(proxyAddr)
	}
}

func probeEndpoints(proxyAddr string) {
	proxyURL, err := url.Parse("http://" + proxyAddr)
	if err != nil {
		return
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
	}

	targets := []string{
		"https://clientsettingscdn.roblox.com/v2/settings/application/PCDesktopClient",
		"https://www.cloudflare.com",
	}

	allSuccess := true

	for _, target := range targets {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil || (resp != nil && resp.StatusCode >= 500) {
			allSuccess = false
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	// Silently cycle strategy if blocked, zero console noise
	if !allSuccess {
		CycleStrategy()
	}
}
