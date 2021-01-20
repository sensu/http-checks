/* Portions of this code are based on and/or derived from the HTTP
   check found in the NCR DevOps Platform nagiosfoundation collection of
   checks found at https://github.com/ncr-devops-platform/nagiosfoundation */

package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PaesslerAG/gval"
	"github.com/itchyny/gojq"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	URL                string
	TrustedCAFile      string
	InsecureSkipVerify bool
	Timeout            int
	Query              string
	Expression         string
	Headers            []string
}

var (
	tlsConfig tls.Config

	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "http-json",
			Short:    "HTTP JSON Check",
			Keyspace: "sensu.io/plugins/http-json/config",
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
			Path:      "query",
			Env:       "",
			Argument:  "query",
			Shorthand: "q",
			Default:   "",
			Usage:     "Query written in jq format",
			Value:     &plugin.Query,
		},
		{
			Path:      "expression",
			Env:       "",
			Argument:  "expression",
			Shorthand: "e",
			Default:   "",
			Usage:     "Expression for comparing result of query",
			Value:     &plugin.Expression,
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
			return sensu.CheckStateWarning, fmt.Errorf("Error loading specified CA file")
		}
		tlsConfig.RootCAs = caCertPool
	}
	tlsConfig.InsecureSkipVerify = plugin.InsecureSkipVerify

	tlsConfig.CipherSuites = corev2.DefaultCipherSuites

	if len(plugin.Query) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--query is required")
	}
	if len(plugin.Expression) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--expression is required")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {

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

	req.Header.Set("Accept", "application/json")
	if len(plugin.Headers) > 0 {
		for _, header := range plugin.Headers {
			headerSplit := strings.SplitN(header, ":", 2)
			req.Header.Set(strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1]))
		}
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

	query, err := gojq.Parse(plugin.Query)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Failed to parse query %q, error: %v", plugin.Query, err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Failed to compile query %q, error: %v", plugin.Query, err)
	}

	var jsonBody interface{}

	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Could not unmarshal response body into JSON: %v", err)
	}

	iter := code.Run(jsonBody)

	var value interface{}

	for {
		var ok bool
		v, ok := iter.Next()
		if !ok {
			// no more iterations
			break
		}
		if _, ok := v.(error); ok {
			// should we output anything here?
			continue
		}
		value = v
	}

	if value == nil {
		fmt.Printf("%s CRITICAL: No value was returned for query %q\n", plugin.PluginConfig.Name, plugin.Query)
		return sensu.CheckStateCritical, nil
	}

	found, err := evaluateExpression(value, plugin.Expression)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("Error evaluating expression: %v", err)
	}
	if found {
		fmt.Printf("%s OK:  The value %v found at %s matched with expression %q and returned true\n", plugin.PluginConfig.Name, value, plugin.Query, plugin.Expression)
		return sensu.CheckStateOK, nil
	}

	fmt.Printf("%s CRITICAL: The value %v found at %s did not match with expression %q and returned false\n", plugin.PluginConfig.Name, value, plugin.Query, plugin.Expression)
	return sensu.CheckStateCritical, nil
}
func evaluateExpression(actualValue interface{}, expression string) (bool, error) {
	evalResult, err := gval.Evaluate("value "+expression, map[string]interface{}{"value": actualValue})
	if err != nil {
		return false, err
	}
	return evalResult.(bool), nil
}
