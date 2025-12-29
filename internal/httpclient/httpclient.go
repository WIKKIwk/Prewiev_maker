package httpclient

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Options struct {
	PreferIPv4 bool
	Timeout    time.Duration
}

func New(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 180 * time.Second
	}

	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if opts.PreferIPv4 {
				return dialer.DialContext(ctx, "tcp4", addr)
			}
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
