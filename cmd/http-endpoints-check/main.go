/* Portions of this code are based on and/or derived from the HTTP
   check found in the NCR DevOps Platform nagiosfoundation collection of
   checks found at https://github.com/ncr-devops-platform/nagiosfoundation */

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/types"
)

// Endpoint represents a http check request
type Endpoint struct {
	URL                string   `json:"url"`
	Headers            []string `json:"header"`
	SearchString       string   `json:"search-string"`
	RedirectOK         bool     `json:"redirect-ok"`
	Timeout            int      `json:"timeout"`
	MTLSKeyFile        string   `json:"mtls-key-file"`
	MTLSCertFile       string   `json:"mtls-cert-file"`
	TrustedCAFile      string   `json:"trusted-ca"`
	InsecureSkipVerify bool     `json:"insecure-skip-verify"`
	CreateEvent        bool     `json:"create-event"`
	EntityName         string   `json:"event-entity-name"`
	CheckName          string   `json:"event-entity-name"`
	Handlers           []string `json:"event-handlers"`
	EventsAPI          string   `json:"events-api"`
	Error              error
	Status             int
	StatusMsg          string
}

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Endpoints          string
	SuppressOKOutput   bool
	DryRun             bool
	URL                string
	SearchString       string
	TrustedCAFile      string
	InsecureSkipVerify bool
	RedirectOK         bool
	Timeout            int
	Headers            []string
	MTLSKeyFile        string
	MTLSCertFile       string
	CreateEvent        bool
	EntityName         string
	CheckName          string
	Handlers           []string
	EventsAPI          string
}

var (
	endpoints []Endpoint
	tlsConfig tls.Config
	plugin    = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "http-check",
			Short:    "HTTP Status/String Check for multiple endpoints",
			Keyspace: "sensu.io/plugins/http-check/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		{
			Path:      "endpoints",
			Env:       "",
			Argument:  "endpoints",
			Shorthand: "e",
			Default:   "",
			Usage:     `An array of http endpoints to check.`,
			Value:     &plugin.Endpoints,
		},
		{
			Path:      "dry-run",
			Env:       "",
			Argument:  "dry-run",
			Shorthand: "n",
			Default:   false,
			Usage:     `Do not actually create events. Output http requests that would have created events instead.`,
			Value:     &plugin.DryRun,
		},
		{
			Path:      "suppress-ok-output",
			Env:       "",
			Argument:  "suppress-ok-output",
			Shorthand: "S",
			Default:   false,
			Usage:     "Aside from overall status, only output failures",
			Value:     &plugin.SuppressOKOutput,
		},

		{
			Path:      "url",
			Env:       "CHECK_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "http://localhost:80/",
			Usage:     "URL to test, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.URL,
		},
		{
			Path:      "search-string",
			Env:       "CHECK_SEARCH_STRING",
			Argument:  "search-string",
			Shorthand: "s",
			Default:   "",
			Usage:     "String to search for, if not provided do status check only, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.SearchString,
		},
		{
			Path:      "insecure-skip-verify",
			Env:       "",
			Argument:  "insecure-skip-verify",
			Shorthand: "i",
			Default:   false,
			Usage:     "Skip TLS certificate verification (not recommended!), can be overridden by endpoint json attribute of same name",
			Value:     &plugin.InsecureSkipVerify,
		},
		{
			Path:      "trusted-ca-file",
			Env:       "",
			Argument:  "trusted-ca-file",
			Shorthand: "t",
			Default:   "",
			Usage:     "TLS CA certificate bundle in PEM format, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.TrustedCAFile,
		},
		{
			Path:      "redirect-ok",
			Env:       "",
			Argument:  "redirect-ok",
			Shorthand: "r",
			Default:   false,
			Usage:     "Allow redirects, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.RedirectOK,
		},
		{
			Path:      "timeout",
			Env:       "",
			Argument:  "timeout",
			Shorthand: "T",
			Default:   15,
			Usage:     "Request timeout in seconds, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.Timeout,
		},
		{
			Path:      "header",
			Env:       "",
			Argument:  "header",
			Shorthand: "H",
			Default:   []string{},
			Usage:     "Additional header(s) to send in check request, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.Headers,
		},
		{
			Path:      "mtls-key-file",
			Env:       "",
			Argument:  "mtls-key-file",
			Shorthand: "K",
			Default:   "",
			Usage:     "Key file for mutual TLS auth in PEM format, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.MTLSKeyFile,
		},
		{
			Path:      "mtls-cert-file",
			Env:       "",
			Argument:  "mtls-cert-file",
			Shorthand: "C",
			Default:   "",
			Usage:     "Certificate file for mutual TLS auth in PEM format, can be overridden by endpoint json attribute of same name",
			Value:     &plugin.MTLSCertFile,
		},
		{
			Path:     "create-event",
			Env:      "",
			Argument: "create-event",
			Default:  false,
			Usage:    "Create event for url, can be overridden by endpoint json attribute of same name",
			Value:    &plugin.CreateEvent,
		},
		{
			Path:     "event-check-name",
			Env:      "",
			Argument: "event-check-name",
			Default:  "",
			Usage:    "Check name to use in generated event, can be overridden by endpoint json attribute of same name",
			Value:    &plugin.CheckName,
		},
		{
			Path:     "event-entity-name",
			Env:      "",
			Argument: "event-entity-name",
			Default:  "",
			Usage:    "Entity name to use in generated event, can be overridden by endpoint json attribute of same name",
			Value:    &plugin.EntityName,
		},
		{
			Path:     "event-handlers",
			Env:      "",
			Argument: "event-handlers",
			Default:  []string{},
			Usage:    "Comma separated list of handlers to use in generated event, can be overridden by endpoint json attribute of same name",
			Value:    &plugin.Handlers,
		},
		{
			Path:     "events-api",
			Env:      "",
			Argument: "events-api",
			Default:  "http://localhost:3031/events",
			Usage:    "Events API endpoint to use when generating events, can be overridden by endpoint json attribute of same name",
			Value:    &plugin.EventsAPI,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	var err error
	if len(plugin.Endpoints) == 0 {
		endpoints, err = parseEndpoints(`[{}]`)
		if err != nil {
			return sensu.CheckStateUnknown, fmt.Errorf("cannot parse config")
		}
	} else {
		endpoints, err = parseEndpoints(plugin.Endpoints)
		if err != nil {
			return sensu.CheckStateUnknown, fmt.Errorf("cannot parse --endpoints string, please check documented examples.")
		}
	}
	if len(endpoints) == 0 {
		return sensu.CheckStateUnknown, fmt.Errorf("no endpoints parsed, please check documented examples.")
	}

	for _, endpoint := range endpoints {

		if len(endpoint.URL) == 0 {
			return sensu.CheckStateWarning, fmt.Errorf("--url or CHECK_URL environment variable is required")
		}
		if len(endpoint.Headers) > 0 {
			for _, header := range endpoint.Headers {
				headerSplit := strings.SplitN(header, ":", 2)
				if len(headerSplit) != 2 {
					return sensu.CheckStateWarning, fmt.Errorf("--header %q value malformed should be \"Header-Name: Header Value\"", header)
				}
			}
		}
		if len(endpoint.TrustedCAFile) > 0 {
			caCertPool, err := corev2.LoadCACerts(endpoint.TrustedCAFile)
			if err != nil {
				return sensu.CheckStateWarning, fmt.Errorf("Error loading specified CA file")
			}
			tlsConfig.RootCAs = caCertPool
		}
		tlsConfig.InsecureSkipVerify = endpoint.InsecureSkipVerify

		tlsConfig.CipherSuites = corev2.DefaultCipherSuites

		if (len(endpoint.MTLSKeyFile) > 0 && len(endpoint.MTLSCertFile) == 0) || (len(endpoint.MTLSCertFile) > 0 && len(endpoint.MTLSKeyFile) == 0) {
			return sensu.CheckStateWarning, fmt.Errorf("mTLS auth requires both --mtls-key-file and --mtls-cert-file")
		}
		if len(endpoint.MTLSKeyFile) > 0 && len(endpoint.MTLSCertFile) > 0 {
			cert, err := tls.LoadX509KeyPair(endpoint.MTLSCertFile, endpoint.MTLSKeyFile)
			if err != nil {
				return sensu.CheckStateWarning, fmt.Errorf("Failed to load mTLS key pair %s/%s: %v", endpoint.MTLSCertFile, endpoint.MTLSKeyFile, err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	client := http.DefaultClient
	for e, endpoint := range endpoints {
		client.Transport = http.DefaultTransport
		client.Timeout = time.Duration(endpoint.Timeout) * time.Second
		if !endpoint.RedirectOK {
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
		}

		checkURL, err := url.Parse(endpoint.URL)
		if len(endpoint.EntityName) == 0 {
			endpoints[e].EntityName = checkURL.Host
		}
		if len(endpoint.CheckName) == 0 {
			// Make a Regex to say we only want letters and numbers
			reg, err := regexp.Compile("[^a-zA-Z0-9]+")
			if err != nil {
				break
			}
			processedString := reg.ReplaceAllString(checkURL.Path, "_")
			if len(processedString) == 0 {
				processedString = "root_path"
			}
			endpoints[e].CheckName = fmt.Sprintf("http_check-%s", processedString)
		}
		if err != nil {
			endpoints[e].Error = err
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: error parsing URL\n",
				plugin.PluginConfig.Name)
			break
		}
		if checkURL.Scheme == "https" {
			client.Transport.(*http.Transport).TLSClientConfig = &tlsConfig
		}

		req, err := http.NewRequest("GET", endpoint.URL, nil)
		if err != nil {
			endpoints[e].Error = err
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: error creating request\n",
				plugin.PluginConfig.Name)
			break
		}

		if len(endpoint.Headers) > 0 {
			for _, header := range endpoint.Headers {
				headerSplit := strings.SplitN(header, ":", 2)
				req.Header.Set(strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1]))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			endpoints[e].Error = err
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: error making request\n",
				plugin.PluginConfig.Name)
			break
		}
		defer resp.Body.Close()

		if err != nil {
			endpoints[e].Error = err
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = "critical"
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: error making request\n",
				plugin.PluginConfig.Name)
			break
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			endpoints[e].Error = err
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = "critical"
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: error reading body\n",
				plugin.PluginConfig.Name)
			break
		}

		if len(endpoint.SearchString) > 0 {
			if strings.Contains(string(body), endpoint.SearchString) {
				endpoints[e].Error = nil
				endpoints[e].Status = sensu.CheckStateOK
				endpoints[e].StatusMsg = fmt.Sprintf(
					"%s OK: found \"%s\" at %s\n",
					plugin.PluginConfig.Name, endpoint.SearchString, resp.Request.URL)
				break
			}
			endpoints[e].Error = nil
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: \"%s\" not found at %s\n",
				plugin.PluginConfig.Name, endpoint.SearchString, resp.Request.URL)
			break
		}

		switch {
		case resp.StatusCode >= http.StatusBadRequest:
			endpoints[e].Error = nil
			endpoints[e].Status = sensu.CheckStateCritical
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s CRITICAL: HTTP Status %v for %s\n",
				plugin.PluginConfig.Name, resp.StatusCode, endpoint.URL)
			break
		// resp.StatusCode will ultimately be 200 for successful redirects
		// so instead we check to see if the current URL matches the requested
		// URL
		case resp.Request.URL.String() != endpoint.URL && endpoint.RedirectOK:
			endpoints[e].Error = nil
			endpoints[e].Status = sensu.CheckStateOK
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s OK: HTTP Status %v for %s (redirect from %s)\n",
				plugin.PluginConfig.Name, resp.StatusCode, resp.Request.URL, endpoint.URL)
			break
		// But, if we've disabled redirects, this should work
		case resp.StatusCode >= http.StatusMultipleChoices:
			var extra string
			redirectURL := resp.Header.Get("Location")
			if len(redirectURL) > 0 {
				extra = fmt.Sprintf(" (redirects to %s)", redirectURL)
			}
			endpoints[e].Error = nil
			endpoints[e].Status = sensu.CheckStateWarning
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s WARNING: HTTP Status %v for %s %s\n",
				plugin.PluginConfig.Name, resp.StatusCode, endpoint.URL, extra)
			break
		case resp.StatusCode == -1:
			endpoints[e].Error = nil
			endpoints[e].Status = sensu.CheckStateUnknown
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s UNKNOWN: HTTP Status %v for %s\n",
				plugin.PluginConfig.Name, resp.StatusCode, endpoint.URL)
			break
		default:
			endpoints[e].Error = nil
			endpoints[e].Status = sensu.CheckStateOK
			endpoints[e].StatusMsg = fmt.Sprintf(
				"%s OK: HTTP Status %v for %s\n",
				plugin.PluginConfig.Name, resp.StatusCode, endpoint.URL)
			break
		}
	}
	overallStatus := 0
	if plugin.DryRun {
		fmt.Printf("\nDry-run:: Events requested:\n")
	}
	for e, endpoint := range endpoints {
		if endpoint.Error == nil && endpoint.CreateEvent {
			endpoints[e].Error = endpoint.generateEvent()
		} else {
			if overallStatus < endpoint.Status {
				overallStatus = endpoint.Status
			}
		}
	}
	if plugin.DryRun {
		fmt.Printf("\nDry-run:: Normal Output:\n")
	}
	var overallError error
	for _, endpoint := range endpoints {
		if endpoint.Error != nil {
			overallError = multierror.Append(overallError, endpoint.Error)
		}
		if (!plugin.SuppressOKOutput && endpoint.Status == 0) || endpoint.Status > 0 {
			fmt.Printf("URL: %s Status: %v Output: %v\n",
				endpoint.URL, endpoint.Status, endpoint.StatusMsg)
		}
	}
	return overallStatus, overallError
}

func parseEndpoints(endpointJSON string) ([]Endpoint, error) {
	endpoints := []Endpoint{}
	err := json.Unmarshal([]byte(endpointJSON), &endpoints)
	if err != nil {
		return []Endpoint{}, err
	}

	return endpoints, nil
}

// Set the defaults for endpoints
func (e *Endpoint) UnmarshalJSON(data []byte) error {
	type endpointAlias Endpoint
	endpoint := &endpointAlias{
		URL:                plugin.URL,
		SearchString:       plugin.SearchString,
		Headers:            plugin.Headers,
		RedirectOK:         plugin.RedirectOK,
		Timeout:            plugin.Timeout,
		MTLSKeyFile:        plugin.MTLSKeyFile,
		MTLSCertFile:       plugin.MTLSCertFile,
		TrustedCAFile:      plugin.TrustedCAFile,
		InsecureSkipVerify: plugin.InsecureSkipVerify,
		CreateEvent:        plugin.CreateEvent,
		EntityName:         plugin.EntityName,
		CheckName:          plugin.CheckName,
		EventsAPI:          plugin.EventsAPI,
		Handlers:           plugin.Handlers,
	}

	_ = json.Unmarshal(data, endpoint)

	*e = Endpoint(*endpoint)
	return nil
}

func (e *Endpoint) generateEvent() error {
	event := types.Event{}
	check := types.Check{}
	entity := types.Entity{}
	event.Check = &check
	event.Entity = &entity
	event.Check.Name = e.CheckName
	event.Check.Status = uint32(e.Status)
	event.Check.Output = e.StatusMsg
	event.Check.Handlers = e.Handlers
	event.Entity.Name = e.EntityName
	eventJSON, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("Create event failed with error %s\n", err)
		return err
	}
	//fmt.Println(string(eventJSON))
	if plugin.DryRun {
		fmt.Printf("URL: %s\n", e.URL)
		fmt.Printf("  Entity Name: %s\n", event.Entity.Name)
		fmt.Printf("  Check Name: %s\n", event.Check.Name)
		fmt.Printf("  Check Status: %v\n", event.Check.Status)
		fmt.Printf("  Check Output: %s\n", event.Check.Output)
		fmt.Printf("  Event API: %s\n  Event Data: %s\n", e.EventsAPI, string(eventJSON))
	} else {
		_, err = http.Post(e.EventsAPI, "application/json", bytes.NewBuffer(eventJSON))
		if err != nil {
			fmt.Printf("The HTTP request to create event failed with error %s\n", err)
			return err
		}
	}
	return nil
}
