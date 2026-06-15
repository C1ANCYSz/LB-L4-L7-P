package utils

import (
	"log"
	"net"
	"net/url"
	"strings"
)

func ResolveUrls(serverUrls []string) []string {
	resolvedURLs := make([]string, len(serverUrls))
	for i, urlStr := range serverUrls {
		resolved, err := ResolveHost(urlStr)
		if err != nil {
			log.Fatalf("failed to resolve %s: %v", urlStr, err)
		}
		resolvedURLs[i] = resolved
		log.Printf("resolved %s → %s", urlStr, resolved)
	}
	return resolvedURLs
}

func ResolveHost(host string) (string, error) {
	// If the string is a URL containing a scheme, parse it and extract the host part
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err == nil && parsed.Host != "" {
			host = parsed.Host
		}
	}

	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return "", err
	}
	addrs, err := net.LookupHost(h)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(addrs[0], port), nil
}
