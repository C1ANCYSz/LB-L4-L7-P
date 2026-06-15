package utils

import (
	"log"
	"net"
)

func ResolveUrls(serverUrls []string) []string {
	resolvedURLs := make([]string, len(serverUrls))
	for i, url := range serverUrls {
		resolved, err := ResolveHost(url)
		if err != nil {
			log.Fatalf("failed to resolve %s: %v", url, err)
		}
		resolvedURLs[i] = resolved
		log.Printf("resolved %s → %s", url, resolved)
	}
	return resolvedURLs
}

func ResolveHost(host string) (string, error) {
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
