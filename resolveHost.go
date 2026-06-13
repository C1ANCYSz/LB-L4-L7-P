package main

import "net"

func resolveHost(host string) (string, error) {
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
