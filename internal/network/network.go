package network

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

const localHost = "127.0.0.1"

type Address struct {
	Interface string
	IP        net.IP
}

type Endpoint struct {
	Interface string `json:"interface"`
	URL       string `json:"url"`
}

type State struct {
	LANEnabled bool       `json:"lan_enabled"`
	LocalURL   string     `json:"local_url"`
	LAN        []Endpoint `json:"lan"`
	Error      string     `json:"error"`
}

func Discover() ([]Address, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := []Address{}
	for _, item := range interfaces {
		if item.Flags&net.FlagUp == 0 || item.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := interfaceAddresses(item)
		if err != nil {
			return nil, err
		}
		result = append(result, addresses...)
	}
	return result, nil
}

func interfaceAddresses(item net.Interface) ([]Address, error) {
	addresses, err := item.Addrs()
	if err != nil {
		return nil, fmt.Errorf("读取网卡 %s：%w", item.Name, err)
	}
	result := []Address{}
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err != nil {
			return nil, fmt.Errorf("解析网卡 %s 地址：%w", item.Name, err)
		}
		if ip.IsGlobalUnicast() {
			result = append(result, Address{Interface: item.Name, IP: ip})
		}
	}
	return result, nil
}

func Enabled(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && !ip.IsLoopback()
}

func Describe(host string, port int, addresses []Address) State {
	state := State{LANEnabled: Enabled(host), LocalURL: baseURL(host, port), LAN: []Endpoint{}}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() {
		return state
	}
	if !ip.IsUnspecified() {
		state.LAN = []Endpoint{{Interface: "指定地址", URL: state.LocalURL}}
		return state
	}
	loopback := localHost
	if ip.To4() == nil {
		loopback = "::1"
	}
	state.LocalURL = baseURL(loopback, port)
	state.LAN = matchingEndpoints(ip, port, addresses)
	return state
}

func matchingEndpoints(host net.IP, port int, addresses []Address) []Endpoint {
	result := []Endpoint{}
	seen := make(map[string]bool)
	for _, address := range addresses {
		ip := address.IP
		if !ip.IsGlobalUnicast() || (ip.To4() == nil) != (host.To4() == nil) {
			continue
		}
		url := baseURL(ip.String(), port)
		if seen[url] {
			continue
		}
		seen[url] = true
		result = append(result, Endpoint{Interface: address.Interface, URL: url})
	}
	slices.SortFunc(result, func(a, b Endpoint) int {
		if a.Interface != b.Interface {
			return strings.Compare(a.Interface, b.Interface)
		}
		return strings.Compare(a.URL, b.URL)
	})
	return result
}

func baseURL(host string, port int) string {
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port))
}
