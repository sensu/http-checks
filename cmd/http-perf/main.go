package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
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
	MTLSKeyFile          string
	MTLSCertFile         string
	Method               string
	Postdata             string
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

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "url",
			Env:       "CHECK_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "http://localhost:80/",
			Usage:     "URL to test",
			Value:     &plugin.URL,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "insecure-skip-verify",
			Env:       "",
			Argument:  "insecure-skip-verify",
			Shorthand: "i",
			Default:   false,
			Usage:     "Skip TLS certificate verification (not recommended!)",
			Value:     &plugin.InsecureSkipVerify,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "trusted-ca-file",
			Env:       "",
			Argument:  "trusted-ca-file",
			Shorthand: "t",
			Default:   "",
			Usage:     "TLS CA certificate bundle in PEM format",
			Value:     &plugin.TrustedCAFile,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "timeout",
			Env:       "",
			Argument:  "timeout",
			Shorthand: "T",
			Default:   15,
			Usage:     "Request timeout in seconds",
			Value:     &plugin.Timeout,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "warning",
			Env:       "",
			Argument:  "warning",
			Shorthand: "w",
			Default:   "1s",
			Usage:     "Warning threshold, can be expressed as seconds or milliseconds (1s = 1000ms)",
			Value:     &plugin.Warning,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "critical",
			Env:       "",
			Argument:  "critical",
			Shorthand: "c",
			Default:   "2s",
			Usage:     "Critical threshold, can be expressed as seconds or milliseconds (1s = 1000ms)",
			Value:     &plugin.Critical,
		},
		&sensu.PluginConfigOption[bool]{
			Path:      "output-in-ms",
			Env:       "",
			Argument:  "output-in-ms",
			Shorthand: "m",
			Default:   false,
			Usage:     "Provide output in milliseconds (default false, display in seconds)",
			Value:     &plugin.OutputInMilliseconds,
		},
		&sensu.SlicePluginConfigOption[string]{
			Path:      "header",
			Env:       "",
			Argument:  "header",
			Shorthand: "H",
			Default:   []string{},
			Usage:     "Additional header(s) to send in check request",
			Value:     &plugin.Headers,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "mtls-key-file",
			Env:       "",
			Argument:  "mtls-key-file",
			Shorthand: "K",
			Default:   "",
			Usage:     "Key file for mutual TLS auth in PEM format",
			Value:     &plugin.MTLSKeyFile,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "mtls-cert-file",
			Env:       "",
			Argument:  "mtls-cert-file",
			Shorthand: "C",
			Default:   "",
			Usage:     "Certificate file for mutual TLS auth in PEM format",
			Value:     &plugin.MTLSCertFile,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "method",
			Argument:  "method",
			Shorthand: "m",
			Default:   "GET",
			Usage:     "Specify http method",
			Value:     &plugin.Method,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "postdata",
			Argument:  "post-data",
			Shorthand: "p",
			Default:   "",
			Usage:     "Data to sent via POST method",
			Value:     &plugin.Postdata,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
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
			return sensu.CheckStateWarning, fmt.Errorf("error loading specified CA file")
		}
		tlsConfig.RootCAs = caCertPool
	}
	tlsConfig.InsecureSkipVerify = plugin.InsecureSkipVerify

	if (len(plugin.MTLSKeyFile) > 0 && len(plugin.MTLSCertFile) == 0) || (len(plugin.MTLSCertFile) > 0 && len(plugin.MTLSKeyFile) == 0) {
		return sensu.CheckStateWarning, fmt.Errorf("mTLS auth requires both --mtls-key-file and --mtls-cert-file")
	}
	if len(plugin.MTLSKeyFile) > 0 && len(plugin.MTLSCertFile) > 0 {
		cert, err := tls.LoadX509KeyPair(plugin.MTLSCertFile, plugin.MTLSKeyFile)
		if err != nil {
			return sensu.CheckStateWarning, fmt.Errorf("failed to load mTLS key pair %s/%s: %v", plugin.MTLSCertFile, plugin.MTLSKeyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	if (plugin.Method == "GET" && len(plugin.Postdata) > 0) || plugin.Method == "POST" && len(plugin.Postdata) < 1 {
		return sensu.CheckStateWarning, fmt.Errorf("malformed POST parameters")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {

	client := http.DefaultClient
	client.Transport = http.DefaultTransport
	client.Timeout = time.Duration(plugin.Timeout) * time.Second

	checkURL, err := url.Parse(plugin.URL)
	if err != nil {
		fmt.Printf("url parse error: %s\n", err)
		return sensu.CheckStateCritical, nil
	}
	if checkURL.Scheme == "https" {
		client.Transport.(*http.Transport).TLSClientConfig = &tlsConfig
	}

	req := &http.Request{}
	if plugin.Method == "POST" {
		rawpost, _ := json.Marshal(plugin.Postdata)
		if err != nil {
			fmt.Printf("failed to parse Postdata: %s\n", err)
			return sensu.CheckStateCritical, nil
		}
		postdata := bytes.NewBuffer(rawpost)
		req, err = http.NewRequest(plugin.Method, plugin.URL, postdata)
		if err != nil {
			fmt.Printf("%s request creation error: %s\n", plugin.Method, err)
			return sensu.CheckStateCritical, nil
		}
	} else {
		req, err = http.NewRequest(plugin.Method, plugin.URL, nil)
		if err != nil {
			fmt.Printf("%s request creation error: %s\n", plugin.Method, err)
			return sensu.CheckStateCritical, nil
		}
	}
	if len(plugin.Headers) > 0 {
		for _, header := range plugin.Headers {
			headerSplit := strings.SplitN(header, ":", 2)
			headerKey := strings.TrimSpace(headerSplit[0])
			headerValue := strings.TrimSpace(headerSplit[1])
			if strings.EqualFold(headerKey, "host") {
				req.Host = headerValue
				continue
			}
			req.Header.Set(headerKey, headerValue)
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
		fmt.Printf("request error: %s\n", err)
		return sensu.CheckStateCritical, nil
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
