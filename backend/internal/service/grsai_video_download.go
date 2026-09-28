package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultGrsaiVideoMaxBytes int64 = 512 << 20
const defaultGrsaiVideoTimeout = 5 * time.Minute

type GrsaiVideoDownloader interface {
	Download(context.Context, string) (io.ReadCloser, int64, error)
}
type grsaiVideoDownloader struct {
	client   *http.Client
	maxBytes int64
	tempDir  string
}

func NewGrsaiVideoDownloader(client *http.Client, maxBytes int64, tempDir string) GrsaiVideoDownloader {
	if maxBytes <= 0 {
		maxBytes = defaultGrsaiVideoMaxBytes
	}
	if client == nil {
		client = &http.Client{Timeout: defaultGrsaiVideoTimeout, Transport: safeVideoTransport()}
	} else {
		clone := *client
		if clone.Timeout <= 0 {
			clone.Timeout = defaultGrsaiVideoTimeout
		}
		client = &clone
	}
	previous := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		if err := validatePublicHTTPSURL(req.URL); err != nil {
			return err
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &grsaiVideoDownloader{client: client, maxBytes: maxBytes, tempDir: tempDir}
}
func safeVideoTransport() *http.Transport {
	d := &net.Dialer{Timeout: 30 * time.Second}
	return &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("video host resolves to private address")
			}
		}
		if len(ips) == 0 {
			return nil, errors.New("video host has no address")
		}
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
}
func validatePublicHTTPSURL(u *url.URL) error {
	if u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return errors.New("video URL must be public HTTPS")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !publicIP(ip) {
		return errors.New("video URL is private")
	}
	return nil
}
func publicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}
func (d *grsaiVideoDownloader) Download(ctx context.Context, raw string) (io.ReadCloser, int64, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, 0, err
	}
	if err = validatePublicHTTPSURL(u); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("download video: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("download video: status %d", resp.StatusCode)
	}
	if resp.ContentLength > d.maxBytes {
		return nil, 0, errors.New("video exceeds size limit")
	}
	f, err := os.CreateTemp(d.tempDir, "grsai-video-*.mp4")
	if err != nil {
		return nil, 0, err
	}
	name := f.Name()
	cleanup := func() { _ = f.Close(); _ = os.Remove(name) }
	n, err := io.Copy(f, io.LimitReader(resp.Body, d.maxBytes+1))
	if err != nil {
		cleanup()
		return nil, 0, err
	}
	if n <= 0 || n > d.maxBytes {
		cleanup()
		return nil, 0, errors.New("invalid video size")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		cleanup()
		return nil, 0, errors.New("truncated video body")
	}
	if _, err = f.Seek(0, 0); err != nil {
		cleanup()
		return nil, 0, err
	}
	head := make([]byte, 12)
	hn, _ := io.ReadFull(f, head)
	if _, err = f.Seek(0, 0); err != nil {
		cleanup()
		return nil, 0, err
	}
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if ct != "video/mp4" && ct != "application/octet-stream" {
		cleanup()
		return nil, 0, fmt.Errorf("invalid video content type %q", ct)
	}
	if hn < 12 || string(head[4:8]) != "ftyp" {
		cleanup()
		return nil, 0, errors.New("invalid MP4 signature")
	}
	return &temporaryVideoFile{File: f, path: name}, n, nil
}

type temporaryVideoFile struct {
	*os.File
	path string
}

func (f *temporaryVideoFile) Close() error { e := f.File.Close(); _ = os.Remove(f.path); return e }
