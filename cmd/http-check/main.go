/* Portions of this code are based on and/or derived from the HTTP
   check found in the NCR DevOps Platform nagiosfoundation collection of
   checks found at https://github.com/ncr-devops-platform/nagiosfoundation */

package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
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
	URL                string
	SearchString       string
	TrustedCAFile      string
	InsecureSkipVerify bool
	RedirectOK         bool
	Timeout            int
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
			Path:      "search-string",
			Env:       "CHECK_SEARCH_STRING",
			Argument:  "search-string",
			Shorthand: "s",
			Default:   "",
			Usage:     "String to search for, if not provided do status check only",
			Value:     &plugin.SearchString,
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
			Path:      "redirect-ok",
			Env:       "",
			Argument:  "redirect-ok",
			Shorthand: "r",
			Default:   false,
			Usage:     "Allow redirects",
			Value:     &plugin.RedirectOK,
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
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	if len(plugin.URL) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--url or CHECK_URL environment variable is required")
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
	if !plugin.RedirectOK {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	}

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

	resp, err := client.Do(req)
	if err != nil {
		return sensu.CheckStateCritical, err
	}

	defer resp.Body.Close()

	if err != nil {
		return sensu.CheckStateCritical, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return sensu.CheckStateCritical, err
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
