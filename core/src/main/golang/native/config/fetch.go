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
)

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

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP in DoH response")
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
		bootstrapCache[host] = node.IPs
		return
	}

	ips := queryDoH(host)
	if len(ips) == 0 {
		log.Warnln("[BootstrapDNS] failed to resolve host %s via DoH", host)
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
	log.Infoln("[BootstrapDNS] successfully resolved %s -> %v via DoH", host, ips)
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

	forEachProviders(rawCfg, func(index int, total int, name string, provider map[string]any, prefix string) {
		bytes, _ := json.Marshal(&Status{
			Action:      "FetchProviders",
			Args:        []string{name},
			Progress:    index,
			MaxProgress: total,
		})

		reportStatus(string(bytes))

		u, uok := provider["url"]
		p, pok := provider["path"]

		if !uok || !pok {
			return
		}

		us, uok := u.(string)
		ps, pok := p.(string)

		if !uok || !pok {
			return
		}

		if _, err := os.Stat(ps); err == nil {
			return
		}

		url, err := U.Parse(us)
		if err != nil {
			return
		}

		if prefix == RULES {
			if pib, uok := provider["path-in-bundle"]; uok {
				if pib, uok := pib.(string); uok && pib != "" {
					// actually, we don't need to extract the file here; the core will do it.
					// however, due to historical reasons, CMFA fetches provider content when loading profile,
					// so we maintain consistency with the old behavior.
					if file, err := RB.Open(pib); err == nil {
						defer file.Close()
						if err := writeFile(ps, file); err == nil {
							return
						}
					}
				}
			}
		}

		_, _ = fetch(url, ps)
	})

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
