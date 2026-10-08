package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	U "net/url"
	"os"
	P "path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cfa/native/app"

	"github.com/metacubex/mihomo/adapter/provider"
	clashHttp "github.com/metacubex/mihomo/component/http"
	"github.com/metacubex/mihomo/component/resolver"
	"github.com/metacubex/mihomo/log"
	RB "github.com/metacubex/mihomo/rules/bundle"
)

type Status struct {
	Action            string   `json:"action"`
	Args              []string `json:"args"`
	Progress          int      `json:"progress"`
	MaxProgress       int      `json:"max"`
	SubUpload         *int64   `json:"subUpload,omitempty"`
	SubDownload       *int64   `json:"subDownload,omitempty"`
	SubTotal          *int64   `json:"subTotal,omitempty"`
	SubExpire         *int64   `json:"subExpire,omitempty"`
	SubUpdateInterval *int64   `json:"subUpdateInterval,omitempty"`
}

type fetchHeader struct {
	SubscriptionUserInfo  string
	ProfileUpdateInterval string
}

var (
	bootstrapHostsLock sync.Mutex
	bootstrapCache     = make(map[string][]netip.Addr)
	dohClient          = &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:   true,
			TLSHandshakeTimeout: 2 * time.Second,
		},
	}
	knownFallbackHosts = map[string][]netip.Addr{
		"vpn.riko.asia": {
			netip.MustParseAddr("104.21.21.175"),
			netip.MustParseAddr("172.67.199.169"),
		},
	}
)

func isFakeOrBogusIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() {
		return true
	}
	if addr.Is4() {
		b := addr.As4()
		// 0.0.0.0/8
		if b[0] == 0 {
			return true
		}
		// 28.0.0.0/8 (Clash default Fake-IP pool)
		if b[0] == 28 {
			return true
		}
		// 198.18.0.0/15 (RFC 2544 benchmark / Standard Clash Fake-IP pool)
		if b[0] == 198 && (b[1] == 18 || b[1] == 19) {
			return true
		}
	} else if addr.Is6() {
		b := addr.As16()
		// 2001:2::/48 (RFC 5180 benchmark / Clash IPv6 Fake-IP pool)
		if b[0] == 0x20 && b[1] == 0x01 && b[2] == 0x00 && b[3] == 0x02 {
			return true
		}
		// fc00::/7 (Unique Local Address - ULA)
		if (b[0] & 0xfe) == 0xfc {
			return true
		}
		// fe80::/10 (Link-Local)
		if b[0] == 0xfe && (b[1]&0xc0) == 0x80 {
			return true
		}
		// 2001:db8::/32 (Documentation)
		if b[0] == 0x20 && b[1] == 0x01 && b[2] == 0x0d && b[3] == 0xb8 {
			return true
		}
	}
	return false
}

func filterValidIPs(ips []netip.Addr) []netip.Addr {
	var valid []netip.Addr
	for _, ip := range ips {
		ip = ip.Unmap()
		if !isFakeOrBogusIP(ip) {
			valid = append(valid, ip)
		}
	}
	return valid
}

func queryUDP(host string) []netip.Addr {
	dnsServers := []string{
		"223.5.5.5:53",
		"119.29.29.29:53",
		"180.76.76.76:53",
		"114.114.114.114:53",
		"1.1.1.1:53",
		"8.8.8.8:53",
	}

	for _, srv := range dnsServers {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: 1500 * time.Millisecond,
				}
				return d.DialContext(ctx, "udp", srv)
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
		ips, err := r.LookupNetIP(ctx, "ip4", host)
		cancel()

		if err == nil && len(ips) > 0 {
			valid := filterValidIPs(ips)
			if len(valid) > 0 {
				log.Infoln("[BootstrapDNS] resolved %s -> %v via UDP (%s)", host, valid, srv)
				return valid
			}
		}
	}
	return nil
}

type dohResponse struct {
	Status int `json:"Status"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func queryDoHEndpoint(ctx context.Context, endpoint string) ([]netip.Addr, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")
	req.Header.Set("User-Agent", "RikoClash-BootstrapDNS/1.0")

	resp, err := dohClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}

	var doh dohResponse
	if err := json.Unmarshal(body, &doh); err != nil {
		return nil, err
	}

	var ips []netip.Addr
	for _, ans := range doh.Answer {
		if ans.Type == 1 || ans.Type == 28 { // A or AAAA
			if addr, err := netip.ParseAddr(ans.Data); err == nil {
				ips = append(ips, addr.Unmap())
			}
		}
	}

	ips = filterValidIPs(ips)
	if len(ips) == 0 {
		return nil, fmt.Errorf("no valid IP in DoH response")
	}

	return ips, nil
}

func queryDoH(host string) []netip.Addr {
	endpoints := []string{
		fmt.Sprintf("https://223.5.5.5/resolve?name=%s&type=A", host),
		fmt.Sprintf("https://223.6.6.6/resolve?name=%s&type=A", host),
		fmt.Sprintf("https://1.1.1.1/dns-query?name=%s&type=A", host),
	}

	for _, ep := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		ips, err := queryDoHEndpoint(ctx, ep)
		cancel()
		if err == nil && len(ips) > 0 {
			log.Infoln("[BootstrapDNS] resolved %s -> %v via DoH (%s)", host, ips, ep)
			return ips
		}
	}
	return nil
}

func resolveBootstrapHost(rawUrl string) {
	u, err := U.Parse(rawUrl)
	if err != nil {
		return
	}
	host := u.Hostname()
	if host == "" {
		return
	}
	if ip := net.ParseIP(host); ip != nil {
		return
	}

	bootstrapHostsLock.Lock()
	defer bootstrapHostsLock.Unlock()

	if cachedIPs, ok := bootstrapCache[host]; ok && len(cachedIPs) > 0 {
		if node, ok := resolver.DefaultHosts.Search(host, false); !ok || len(node.IPs) == 0 {
			if hv, err := resolver.NewHostValueByIPs(cachedIPs); err == nil {
				_ = resolver.DefaultHosts.Insert(host, hv)
			}
		}
		return
	}

	if node, ok := resolver.DefaultHosts.Search(host, false); ok && len(node.IPs) > 0 {
		valid := filterValidIPs(node.IPs)
		if len(valid) > 0 {
			bootstrapCache[host] = valid
			return
		}
	}

	// 1. Direct DoH query over TLS 443 (immune to UDP port 53 transparent proxying / Fake-IP poisoning)
	ips := queryDoH(host)

	// 2. Fallback to direct UDP DNS query if DoH fails
	if len(ips) == 0 {
		ips = queryUDP(host)
	}

	// 3. Fallback to known static Anycast IPs
	if len(ips) == 0 {
		if fallback, ok := knownFallbackHosts[host]; ok && len(fallback) > 0 {
			ips = fallback
			log.Infoln("[BootstrapDNS] using static fallback Anycast IPs for %s -> %v", host, ips)
		}
	}

	if len(ips) == 0 {
		log.Warnln("[BootstrapDNS] failed to resolve host %s via all bootstrap methods", host)
		return
	}

	hv, err := resolver.NewHostValueByIPs(ips)
	if err != nil {
		log.Warnln("[BootstrapDNS] invalid HostValue for %s: %s", host, err)
		return
	}

	if err := resolver.DefaultHosts.Insert(host, hv); err != nil {
		log.Warnln("[BootstrapDNS] insert DefaultHosts failed for %s: %s", host, err)
		return
	}

	bootstrapCache[host] = ips
	log.Infoln("[BootstrapDNS] successfully resolved %s -> %v via bootstrap DNS", host, ips)
}

func GetBootstrapHosts() map[string][]string {
	bootstrapHostsLock.Lock()
	defer bootstrapHostsLock.Unlock()

	res := make(map[string][]string, len(bootstrapCache))
	for h, ips := range bootstrapCache {
		strIps := make([]string, 0, len(ips))
		for _, ip := range ips {
			strIps = append(strIps, ip.String())
		}
		res[h] = strIps
	}
	return res
}

func openUrl(ctx context.Context, url string) (io.ReadCloser, fetchHeader, error) {
	resolveBootstrapHost(url)

	response, err := clashHttp.HttpRequest(ctx, url, http.MethodGet, http.Header{"User-Agent": {"ClashMetaForAndroid/" + app.VersionName()}}, nil)

	if err != nil {
		return nil, fetchHeader{}, err
	}

	return response.Body, fetchHeader{
		SubscriptionUserInfo:  response.Header.Get("subscription-userinfo"),
		ProfileUpdateInterval: response.Header.Get("profile-update-interval"),
	}, nil
}

func openContent(url string) (io.ReadCloser, error) {
	return app.OpenContent(url)
}

func fetch(url *U.URL, file string) (fetchHeader, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var reader io.ReadCloser
	var header fetchHeader
	var err error

	switch url.Scheme {
	case "http", "https":
		reader, header, err = openUrl(ctx, url.String())
	case "content":
		reader, err = openContent(url.String())
	default:
		err = fmt.Errorf("unsupported scheme %s of %s", url.Scheme, url)
	}

	if err != nil {
		return fetchHeader{}, err
	}

	defer reader.Close()

	return header, writeFile(file, reader)
}

func writeFile(file string, reader io.Reader) error {
	_ = os.MkdirAll(P.Dir(file), 0700)

	f, err := os.OpenFile(file, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = io.Copy(f, reader)
	if err != nil {
		_ = os.Remove(file)
	}

	return err
}

func parseProfileUpdateInterval(value string) (int64, bool) {
	hours, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}

	if hours <= 0 {
		return 0, true
	}

	interval := time.Duration(hours) * time.Hour
	if interval < 15*time.Minute {
		interval = 15 * time.Minute
	}

	return int64(interval / time.Millisecond), true
}

func reportSubscriptionInfo(header fetchHeader, reportStatus func(string)) {
	userinfo := header.SubscriptionUserInfo
	updateIntervalHeader := header.ProfileUpdateInterval
	if userinfo == "" && updateIntervalHeader == "" {
		return
	}

	status := Status{
		Action:      "SubscriptionInfo",
		Args:        []string{},
		Progress:    -1,
		MaxProgress: -1,
	}

	if userinfo != "" {
		info := provider.NewSubscriptionInfo(userinfo)
		expire := info.Expire * 1000
		status.SubUpload = &info.Upload
		status.SubDownload = &info.Download
		status.SubTotal = &info.Total
		status.SubExpire = &expire
	}

	if interval, ok := parseProfileUpdateInterval(updateIntervalHeader); ok {
		status.SubUpdateInterval = &interval
	}

	bytes, _ := json.Marshal(&status)
	reportStatus(string(bytes))
}

func FetchAndValid(
	path string,
	url string,
	force bool,
	reportStatus func(string),
) error {
	configPath := P.Join(path, "config.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) || force {
		url, err := U.Parse(url)
		if err != nil {
			return err
		}

		bytes, _ := json.Marshal(&Status{
			Action:      "FetchConfiguration",
			Args:        []string{url.Host},
			Progress:    -1,
			MaxProgress: -1,
		})

		reportStatus(string(bytes))

		header, err := fetch(url, configPath)
		if err != nil {
			return err
		}

		reportSubscriptionInfo(header, reportStatus)
	}

	defer runtime.GC()

	rawCfg, err := UnmarshalAndPatch(path)
	if err != nil {
		return err
	}

	type providerTask struct {
		name     string
		provider map[string]any
		prefix   string
	}

	var tasks []providerTask
	forEachProviders(rawCfg, func(index int, total int, name string, provider map[string]any, prefix string) {
		tasks = append(tasks, providerTask{
			name:     name,
			provider: provider,
			prefix:   prefix,
		})
	})

	total := len(tasks)
	if total > 0 {
		var (
			completedCount int32
			statusMu       sync.Mutex
			wg             sync.WaitGroup
			sem            = make(chan struct{}, 4)
		)

		for _, task := range tasks {
			t := task
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				u, uok := t.provider["url"]
				p, pok := t.provider["path"]
				if !uok || !pok {
					c := int(atomic.AddInt32(&completedCount, 1))
					statusMu.Lock()
					b, _ := json.Marshal(&Status{
						Action:      "FetchProviders",
						Args:        []string{t.name},
						Progress:    c,
						MaxProgress: total,
					})
					reportStatus(string(b))
					statusMu.Unlock()
					return
				}

				us, uok := u.(string)
				ps, pok := p.(string)
				if !uok || !pok {
					c := int(atomic.AddInt32(&completedCount, 1))
					statusMu.Lock()
					b, _ := json.Marshal(&Status{
						Action:      "FetchProviders",
						Args:        []string{t.name},
						Progress:    c,
						MaxProgress: total,
					})
					reportStatus(string(b))
					statusMu.Unlock()
					return
				}

				if _, err := os.Stat(ps); err == nil {
					c := int(atomic.AddInt32(&completedCount, 1))
					statusMu.Lock()
					b, _ := json.Marshal(&Status{
						Action:      "FetchProviders",
						Args:        []string{t.name},
						Progress:    c,
						MaxProgress: total,
					})
					reportStatus(string(b))
					statusMu.Unlock()
					return
				}

				url, err := U.Parse(us)
				if err != nil {
					c := int(atomic.AddInt32(&completedCount, 1))
					statusMu.Lock()
					b, _ := json.Marshal(&Status{
						Action:      "FetchProviders",
						Args:        []string{t.name},
						Progress:    c,
						MaxProgress: total,
					})
					reportStatus(string(b))
					statusMu.Unlock()
					return
				}

				if t.prefix == RULES {
					if pib, uok := t.provider["path-in-bundle"]; uok {
						if pib, uok := pib.(string); uok && pib != "" {
							if file, err := RB.Open(pib); err == nil {
								defer file.Close()
								if err := writeFile(ps, file); err == nil {
									c := int(atomic.AddInt32(&completedCount, 1))
									statusMu.Lock()
									b, _ := json.Marshal(&Status{
										Action:      "FetchProviders",
										Args:        []string{t.name},
										Progress:    c,
										MaxProgress: total,
									})
									reportStatus(string(b))
									statusMu.Unlock()
									return
								}
							}
						}
					}
				}

				statusMu.Lock()
				b, _ := json.Marshal(&Status{
					Action:      "FetchProviders",
					Args:        []string{t.name},
					Progress:    int(atomic.LoadInt32(&completedCount)),
					MaxProgress: total,
				})
				reportStatus(string(b))
				statusMu.Unlock()

				_, _ = fetch(url, ps)

				c := int(atomic.AddInt32(&completedCount, 1))
				statusMu.Lock()
				b, _ = json.Marshal(&Status{
					Action:      "FetchProviders",
					Args:        []string{t.name},
					Progress:    c,
					MaxProgress: total,
				})
				reportStatus(string(b))
				statusMu.Unlock()
			}()
		}
		wg.Wait()
	}

	bytes, _ := json.Marshal(&Status{
		Action:      "Verifying",
		Args:        []string{},
		Progress:    0xffff,
		MaxProgress: 0xffff,
	})

	reportStatus(string(bytes))

	cfg, err := Parse(rawCfg)
	if err != nil {
		return err
	}

	destroyProviders(cfg)

	return nil
}
