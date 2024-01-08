/* Portions of this code are based on and/or derived from the HTTP
   check found in the NCR DevOps Platform nagiosfoundation collection of
   checks found at https://github.com/ncr-devops-platform/nagiosfoundation */

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	URL                string
	SearchString       string
	TrustedCAFile      string
	InsecureSkipVerify bool
	RedirectOK         bool
	Timeout            int
	Headers            []string
	MTLSKeyFile        string
	MTLSCertFile       string
	Method             string
	Postdata           string
}

var (
	tlsConfig tls.Config

	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "http-check",
			Short:    "HTTP Status/String Check",
			Keyspace: "sensu.io/plugins/http-check/config",
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
		&sensu.PluginConfigOption[string]{
			Path:      "search-string",
			Env:       "CHECK_SEARCH_STRING",
			Argument:  "search-string",
			Shorthand: "s",
			Default:   "",
			Usage:     "String to search for, if not provided do status check only",
			Value:     &plugin.SearchString,
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
		&sensu.PluginConfigOption[bool]{
			Path:      "redirect-ok",
			Env:       "",
			Argument:  "redirect-ok",
			Shorthand: "r",
			Default:   false,
			Usage:     "Allow redirects",
			Value:     &plugin.RedirectOK,
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
	if !plugin.RedirectOK {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	}

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
			fmt.Printf("%s request creation error: %s\n",plugin.Method, err)
			return sensu.CheckStateCritical, nil
		}
	} else {
		req, err = http.NewRequest(plugin.Method, plugin.URL, nil)
		if err != nil {
			fmt.Printf("%s request creation error: %s\n",plugin.Method, err)
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

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("request error: %s\n", err)
		return sensu.CheckStateCritical, nil
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("response body read error: %s\n", err)
		return sensu.CheckStateCritical, nil
	}

	if len(plugin.SearchString) > 0 {
		if strings.Contains(string(body), plugin.SearchString) {
			fmt.Printf("%s OK: found \"%s\" at %s\n", plugin.PluginConfig.Name, plugin.SearchString, resp.Request.URL)
			return sensu.CheckStateOK, nil
		}
		fmt.Printf("%s CRITICAL: \"%s\" not found at %s\n", plugin.PluginConfig.Name, plugin.SearchString, resp.Request.URL)
		return sensu.CheckStateCritical, nil
	}

	switch {
	case resp.StatusCode >= http.StatusBadRequest:
		fmt.Printf("%s CRITICAL: HTTP Status %v for %s\n", plugin.PluginConfig.Name, resp.StatusCode, plugin.URL)
		return sensu.CheckStateCritical, nil
	// resp.StatusCode will ultimately be 200 for successful redirects
	// so instead we check to see if the current URL matches the requested
	// URL
	case resp.Request.URL.String() != plugin.URL && plugin.RedirectOK:
		fmt.Printf("%s OK: HTTP Status %v for %s (redirect from %s)\n", plugin.PluginConfig.Name, resp.StatusCode, resp.Request.URL, plugin.URL)
		return sensu.CheckStateOK, nil
	// But, if we've disabled redirects, this should work
	case resp.StatusCode >= http.StatusMultipleChoices:
		var extra string
		redirectURL := resp.Header.Get("Location")
		if len(redirectURL) > 0 {
			extra = fmt.Sprintf(" (redirects to %s)", redirectURL)
		}
		fmt.Printf("%s WARNING: HTTP Status %v for %s %s\n", plugin.PluginConfig.Name, resp.StatusCode, plugin.URL, extra)
		return sensu.CheckStateWarning, nil
	case resp.StatusCode == -1:
		fmt.Printf("%s UNKNOWN: HTTP Status %v for %s\n", plugin.PluginConfig.Name, resp.StatusCode, plugin.URL)
		return sensu.CheckStateUnknown, nil
	default:
		fmt.Printf("%s OK: HTTP Status %v for %s\n", plugin.PluginConfig.Name, resp.StatusCode, plugin.URL)
		return sensu.CheckStateOK, nil
	}
}
