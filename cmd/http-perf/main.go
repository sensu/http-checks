package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/types"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	URL                  string
	TrustedCAFile        string
	InsecureSkipVerify   bool
	Timeout              int
	Warning              string
	Critical             string
	OutputInMilliseconds bool
	Headers              []string
}

var (
	tlsConfig         tls.Config
	warning, critical time.Duration

	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "http-perf",
			Short:    "HTTP Performance Check",
			Keyspace: "sensu.io/plugins/http-perf/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		{
			Path:      "url",
			Env:       "CHECK_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "http://localhost:80/",
			Usage:     "URL to test",
			Value:     &plugin.URL,
		},
		{
			Path:      "insecure-skip-verify",
			Env:       "",
			Argument:  "insecure-skip-verify",
			Shorthand: "i",
			Default:   false,
			Usage:     "Skip TLS certificate verification (not recommended!)",
			Value:     &plugin.InsecureSkipVerify,
		},
		{
			Path:      "trusted-ca-file",
			Env:       "",
			Argument:  "trusted-ca-file",
			Shorthand: "t",
			Default:   "",
			Usage:     "TLS CA certificate bundle in PEM format",
			Value:     &plugin.TrustedCAFile,
		},
		{
			Path:      "timeout",
			Env:       "",
			Argument:  "timeout",
			Shorthand: "T",
			Default:   15,
			Usage:     "Request timeout in seconds",
			Value:     &plugin.Timeout,
		},
		{
			Path:      "warning",
			Env:       "",
			Argument:  "warning",
			Shorthand: "w",
			Default:   "1s",
			Usage:     "Warning threshold, can be expressed as seconds or milliseconds (1s = 1000ms)",
			Value:     &plugin.Warning,
		},
		{
			Path:      "critical",
			Env:       "",
			Argument:  "critical",
			Shorthand: "c",
			Default:   "2s",
			Usage:     "Critical threshold, can be expressed as seconds or milliseconds (1s = 1000ms)",
			Value:     &plugin.Critical,
		},
		{
			Path:      "output-in-ms",
			Env:       "",
			Argument:  "output-in-ms",
			Shorthand: "m",
			Default:   false,
			Usage:     "Provide output in milliseconds (default false, display in seconds)",
			Value:     &plugin.OutputInMilliseconds,
		},
		{
			Path:      "header",
			Env:       "",
			Argument:  "header",
			Shorthand: "H",
			Default:   []string{},
			Usage:     "Additional header(s) to send in check request",
			Value:     &plugin.Headers,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	var err error

	if len(plugin.URL) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--url or CHECK_URL environment variable is required")
	}
	if len(plugin.Headers) > 0 {
		for _, header := range plugin.Headers {
			headerSplit := strings.SplitN(header, ":", 2)
			if len(headerSplit) != 2 {
				return sensu.CheckStateWarning, fmt.Errorf("--header %q value malformed should be \"Header-Name: Header Value\"", header)
			}
		}
	}
	warning, err = time.ParseDuration(plugin.Warning)
	if err != nil {
		return sensu.CheckStateCritical, err
	}
	critical, err = time.ParseDuration(plugin.Critical)
	if err != nil {
		return sensu.CheckStateCritical, err
	}
	if len(plugin.TrustedCAFile) > 0 {
		caCertPool, err := corev2.LoadCACerts(plugin.TrustedCAFile)
		if err != nil {
			return sensu.CheckStateWarning, fmt.Errorf("Error loading specified CA file")
		}
		tlsConfig.RootCAs = caCertPool
	}
	tlsConfig.InsecureSkipVerify = plugin.InsecureSkipVerify

	tlsConfig.CipherSuites = corev2.DefaultCipherSuites

	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {

	client := http.DefaultClient
	client.Transport = http.DefaultTransport
	client.Timeout = time.Duration(plugin.Timeout) * time.Second

	checkURL, err := url.Parse(plugin.URL)
	if err != nil {
		return sensu.CheckStateCritical, err
	}
	if checkURL.Scheme == "https" {
		client.Transport.(*http.Transport).TLSClientConfig = &tlsConfig
	}

	req, err := http.NewRequest("GET", plugin.URL, nil)
	if err != nil {
		return sensu.CheckStateCritical, err
	}
	if len(plugin.Headers) > 0 {
		for _, header := range plugin.Headers {
			headerSplit := strings.SplitN(header, ":", 2)
			req.Header.Set(strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1]))
		}
	}

	var (
		start                time.Time
		connect              time.Time
		dns                  time.Time
		tlsHandshake         time.Time
		totalRequestDuration time.Duration
		firstByteDuration    time.Duration
		connectDuration      time.Duration
		dnsDuration          time.Duration
		tlsHandshakeDuration time.Duration
		output               string
		perfdata             string
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			dnsDuration = time.Since(dns)
		},

		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			tlsHandshakeDuration = time.Since(tlsHandshake)
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			connectDuration = time.Since(connect)
		},

		GotFirstResponseByte: func() {
			firstByteDuration = time.Since(start)
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start = time.Now()
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return sensu.CheckStateCritical, err
	}
	totalRequestDuration = time.Since(start)

	defer resp.Body.Close()

	if plugin.OutputInMilliseconds {
		output = fmt.Sprintf("%dms", totalRequestDuration.Milliseconds())
		perfdata = fmt.Sprintf("dns_duration=%d, tls_handshake_duration=%d, connect_duration=%d, first_byte_duration=%d, total_request_duration=%d", dnsDuration.Milliseconds(), tlsHandshakeDuration.Milliseconds(), connectDuration.Milliseconds(), firstByteDuration.Milliseconds(), totalRequestDuration.Milliseconds())
	} else {
		output = fmt.Sprintf("%0.6fs", totalRequestDuration.Seconds())
		perfdata = fmt.Sprintf("dns_duration=%0.6f, tls_handshake_duration=%0.6f, connect_duration=%0.6f, first_byte_duration=%0.6f, total_request_duration=%0.6f", dnsDuration.Seconds(), tlsHandshakeDuration.Seconds(), connectDuration.Seconds(), firstByteDuration.Seconds(), totalRequestDuration.Seconds())
	}
	if totalRequestDuration > critical {
		fmt.Printf("http-perf CRITICAL: %s | %s\n", output, perfdata)
		return sensu.CheckStateCritical, nil
	} else if totalRequestDuration > warning {
		fmt.Printf("http-perf WARNING: %s | %s\n", output, perfdata)
		return sensu.CheckStateWarning, nil
	}

	fmt.Printf("http-perf OK: %s | %s\n", output, perfdata)

	return sensu.CheckStateOK, nil
}
