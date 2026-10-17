package main

import (
	"os"
	"strings"
)

const antidpiBlockHeader = "# --- ANTIDPI BYPASS START ---"
const antidpiBlockFooter = "# --- ANTIDPI BYPASS END ---"
const legacyRobloxBlockHeader = "# --- ANTIDPI ROBLOX BYPASS START ---"
const legacyRobloxBlockFooter = "# --- ANTIDPI ROBLOX BYPASS END ---"

var hostsEntries = []string{
	// Roblox Web & Client
	"127.0.0.1 roblox.com",
	"127.0.0.1 www.roblox.com",
	"127.0.0.1 apis.roblox.com",
	"127.0.0.1 auth.roblox.com",
	"127.0.0.1 users.roblox.com",
	"127.0.0.1 ecsv2.roblox.com",
	"127.0.0.1 realtime-signalr.roblox.com",
	"127.0.0.1 notifications.roblox.com",
	"127.0.0.1 clientsettingscdn.roblox.com",
	"127.0.0.1 clientsettings.roblox.com",
	"127.0.0.1 gamejoin.roblox.com",
	"127.0.0.1 assetgame.roblox.com",
	"127.0.0.1 versioncompatibility.api.roblox.com",
	"127.0.0.1 setup.rbxcdn.com",
	"127.0.0.1 contentstore.roblox.com",
	"127.0.0.1 locale.roblox.com",
	"127.0.0.1 voice.roblox.com",
	"127.0.0.1 thumbnails.roblox.com",
	"127.0.0.1 avatar.roblox.com",
	"127.0.0.1 games.roblox.com",
	"127.0.0.1 presence.roblox.com",
	"127.0.0.1 friends.roblox.com",
	"127.0.0.1 assetdelivery.roblox.com",
	"127.0.0.1 contentdelivery.roblox.com",
	"127.0.0.1 privatemessages.roblox.com",
	"127.0.0.1 contacts.roblox.com",
	"127.0.0.1 metrics.roblox.com",
	"127.0.0.1 chat.roblox.com",
	"127.0.0.1 groups.roblox.com",
	"127.0.0.1 economy.roblox.com",
	"127.0.0.1 inventory.roblox.com",
	"127.0.0.1 develop.roblox.com",
	"127.0.0.1 badges.roblox.com",
	"127.0.0.1 accountsettings.roblox.com",
	"127.0.0.1 accountinformation.roblox.com",
	"127.0.0.1 ephemeralcounters.api.roblox.com",
	"127.0.0.1 gameinternationalization.roblox.com",
	"127.0.0.1 devforum.roblox.com",
	"127.0.0.1 create.roblox.com",
	"127.0.0.1 talent.roblox.com",
	"127.0.0.1 education.roblox.com",
	"127.0.0.1 help.roblox.com",
	"127.0.0.1 en.help.roblox.com",
	"127.0.0.1 corp.roblox.com",
	"127.0.0.1 rbx.com",
	"127.0.0.1 www.rbx.com",
	"127.0.0.1 robloxlabs.com",

	// Roblox Catalog, Item Configuration & Matchmaking
	"127.0.0.1 catalog.roblox.com",
	"127.0.0.1 itemconfiguration.roblox.com",
	"127.0.0.1 itemconfiguration.api.roblox.com",
	"127.0.0.1 matchmaking.roblox.com",
	"127.0.0.1 client-telemetry.roblox.com",
	"127.0.0.1 trades.roblox.com",
	"127.0.0.1 premiumfeatures.roblox.com",
	"127.0.0.1 translations.roblox.com",
	"127.0.0.1 followings.roblox.com",
	"127.0.0.1 engagement.roblox.com",
	"127.0.0.1 private-messages.roblox.com",
	"127.0.0.1 share.roblox.com",

	// Roblox 3D Avatar Rendering & Asset CDNs
	"127.0.0.1 tr.rbxcdn.com",
	"127.0.0.1 images.rbxcdn.com",
	"127.0.0.1 static.rbxcdn.com",
	"127.0.0.1 js.rbxcdn.com",
	"127.0.0.1 css.rbxcdn.com",
	"127.0.0.1 c0.rbxcdn.com",
	"127.0.0.1 c1.rbxcdn.com",
	"127.0.0.1 c2.rbxcdn.com",
	"127.0.0.1 c3.rbxcdn.com",
	"127.0.0.1 c4.rbxcdn.com",
	"127.0.0.1 c5.rbxcdn.com",
	"127.0.0.1 c6.rbxcdn.com",
	"127.0.0.1 c7.rbxcdn.com",
	"127.0.0.1 t0.rbxcdn.com",
	"127.0.0.1 t1.rbxcdn.com",
	"127.0.0.1 t2.rbxcdn.com",
	"127.0.0.1 t3.rbxcdn.com",
	"127.0.0.1 t4.rbxcdn.com",
	"127.0.0.1 t5.rbxcdn.com",
	"127.0.0.1 t6.rbxcdn.com",
	"127.0.0.1 t7.rbxcdn.com",

	// Discord Web, Client & Gateway
	"127.0.0.1 discord.com",
	"127.0.0.1 www.discord.com",
	"127.0.0.1 support.discord.com",
	"127.0.0.1 support-dev.discord.com",
	"127.0.0.1 blog.discord.com",
	"127.0.0.1 status.discord.com",
	"127.0.0.1 canary.discord.com",
	"127.0.0.1 ptb.discord.com",
	"127.0.0.1 app.discord.com",
	"127.0.0.1 auth.discord.com",
	"127.0.0.1 login.discord.com",
	"127.0.0.1 assets.discord.com",
	"127.0.0.1 creative.discord.com",
	"127.0.0.1 creative.discordapp.com",
	"127.0.0.1 activities.discord.com",
	"127.0.0.1 activities-static.discord.com",
	"127.0.0.1 streamkit.discord.com",
	"127.0.0.1 safety.discord.com",
	"127.0.0.1 developer.discord.com",
	"127.0.0.1 admin.discord.com",
	"127.0.0.1 updates.discord.com",
	"127.0.0.1 dl.discord.com",
	"127.0.0.1 cdn.discord.com",
	"127.0.0.1 media.discord.com",
	"127.0.0.1 router.discord.com",
	"127.0.0.1 voice.discord.com",
	"127.0.0.1 discordapp.com",
	"127.0.0.1 www.discordapp.com",
	"127.0.0.1 support.discordapp.com",
	"127.0.0.1 status.discordapp.com",
	"127.0.0.1 canary.discordapp.com",
	"127.0.0.1 ptb.discordapp.com",
	"127.0.0.1 gateway.discord.gg",
	"127.0.0.1 remote-auth-gateway.discord.gg",
	"127.0.0.1 remote-auth.discord.gg",
	"127.0.0.1 qr.discord.gg",
	"127.0.0.1 voice-gateway.discord.gg",
	"127.0.0.1 media.discord.gg",
	"127.0.0.1 cdn.discordapp.com",
	"127.0.0.1 media.discordapp.net",
	"127.0.0.1 images-ext-0.discordapp.net",
	"127.0.0.1 images-ext-1.discordapp.net",
	"127.0.0.1 images-ext-2.discordapp.net",
	"127.0.0.1 images-ext-3.discordapp.net",
	"127.0.0.1 images-ext-4.discordapp.net",
	"127.0.0.1 router.discordapp.net",
	"127.0.0.1 updates.discordapp.com",
	"127.0.0.1 dl.discordapp.net",
	"127.0.0.1 dl-canary.discordapp.net",
	"127.0.0.1 dl-ptb.discordapp.net",
	"127.0.0.1 discord.gg",
	"127.0.0.1 discord.media",
	"127.0.0.1 latency.discord.media",
	"127.0.0.1 rtc.discord.media",
	"127.0.0.1 dis.gd",
	"127.0.0.1 discordstatus.com",
	"127.0.0.1 discordmerch.com",
	"127.0.0.1 discord.co",

	// Popular Blocked Services (Turkey ISP DNS Sinkhole Bypass)
	"127.0.0.1 wikileaks.org",
	"127.0.0.1 www.wikileaks.org",
	"127.0.0.1 pastebin.com",
	"127.0.0.1 www.pastebin.com",
	"127.0.0.1 imgur.com",
	"127.0.0.1 i.imgur.com",
	"127.0.0.1 m.imgur.com",
	"127.0.0.1 wattpad.com",
	"127.0.0.1 www.wattpad.com",
	"127.0.0.1 eksisozluk.com",
	"127.0.0.1 www.eksisozluk.com",
	"127.0.0.1 eksisozluk1923.com",
	"127.0.0.1 eksisozluk2023.com",
	"127.0.0.1 proton.me",
	"127.0.0.1 mail.proton.me",
	"127.0.0.1 protonmail.com",
}

// EnableHosts injects direct unblocked Roblox and Discord IPs into hosts file to bypass ISP DNS sinkholes
func EnableHosts() {
	p := getHostsPath()
	content, err := os.ReadFile(p)
	if err != nil {
		return
	}
	text := cleanHostsText(string(content))

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(text, "\r\n"))
	sb.WriteString("\n\n")
	sb.WriteString(antidpiBlockHeader)
	sb.WriteString("\n")
	for _, entry := range hostsEntries {
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
	sb.WriteString(antidpiBlockFooter)
	sb.WriteString("\n")

	_ = os.WriteFile(p, []byte(sb.String()), 0644)
	FlushDNS()
}

// DisableHosts removes injected Roblox and Discord IPs from hosts file
func DisableHosts() {
	p := getHostsPath()
	content, err := os.ReadFile(p)
	if err != nil {
		return
	}
	cleaned := cleanHostsText(string(content))
	_ = os.WriteFile(p, []byte(cleaned), 0644)
	FlushDNS()
}

func EnableRobloxHosts()  { EnableHosts() }
func DisableRobloxHosts() { DisableHosts() }

func cleanBlock(text, header, footer string) string {
	startIdx := strings.Index(text, header)
	if startIdx == -1 {
		return text
	}
	endIdx := strings.Index(text, footer)
	if endIdx == -1 {
		return text[:startIdx]
	}
	endIdx += len(footer)
	return strings.TrimRight(text[:startIdx], "\r\n") + "\n" + strings.TrimLeft(text[endIdx:], "\r\n")
}

func cleanHostsText(text string) string {
	text = cleanBlock(text, antidpiBlockHeader, antidpiBlockFooter)
	text = cleanBlock(text, legacyRobloxBlockHeader, legacyRobloxBlockFooter)
	return text
}
