[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/sensu/http-checks)
![Go Test](https://github.com/sensu/http-checks/workflows/Go%20Test/badge.svg)
![goreleaser](https://github.com/sensu/http-checks/workflows/goreleaser/badge.svg)

# http-checks

## Table of Contents
- [Overview](#overview)
  - [Attribution](#attribution)
  - [Checks](#checks)
- [Usage examples](#usage-examples)
  - [http-check](#http-check)
  - [http-perf](#http-perf)
  - [http-json](#http-json)
  - [http-get](#http-get)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Check definitions](#check-definition)
- [Installation from source](#installation-from-source)
- [Contributing](#contributing)

## Overview

The http-checks is a collection of [Sensu Checks][1] that providing monitoring
and metrics for HTTP based services.

### Attribution

Portions of http-check and http-json are based on and/or derived from the HTTP
check found in the [NCR DevOps Platform nagiosfoundation][5] collection of
checks.

### Checks

This collection contains the following checks:

* `http-check` - for checking HTTP status or searching for a string in the
response body
* `http-perf` - for checking HTTP performance by measuring response times,
provides metrics in nagios_perfdata format
* `http-json` - for querying JSON output from an HTTP request
* `http-get` - for fetching metrics from HTTP sources

## Usage examples

### http-check

#### Help output

```
HTTP Status/String Check

Usage:
  http-check [flags]
  http-check [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -u, --url string               URL to test (default "http://localhost:80/")
  -s, --search-string string     String to search for, if not provided do status check only
  -r, --redirect-ok              Allow redirects
  -T, --timeout int              Request timeout in seconds (default 15)
  -H, --header strings           Additional header(s) to send in check request
  -C, --mtls-cert-file string    Certificate file for mutual TLS auth in PEM format
  -K, --mtls-key-file string     Key file for mutual TLS auth in PEM format
  -t, --trusted-ca-file string   TLS CA certificate bundle in PEM format
  -i, --insecure-skip-verify     Skip TLS certificate verification (not recommended!)
  -h, --help                     help for http-check

Use "http-check [command] --help" for more information about a command.
```

#### Example(s)

```
http-check --url https://sensu.io --search-string Monitoring
http-check OK: found "Monitoring" at https://sensu.io

http-check --url https://sensu.io --search-string droids
http-check CRITICAL: "droids" not found at https://sensu.io

http-check --url https://sensu.io
http-check OK: HTTP Status 200 for https://sensu.io

http-check --url https://sensu.io/notfound
http-check WARNING: HTTP Status 301 for https://sensu.io/notfound  (redirects to /oops)

http-check --url https://sensu.io/notfound --redirect-ok
http-check OK: HTTP Status 200 for https://sensu.io/oops (redirect from https://sensu.io/notfound)

http-check --url https://discourse.sensu.io/notfound
http-check CRITICAL: HTTP Status 404 for https://discourse.sensu.io/notfound

http-check --url http://localhost:8000/health --header "Origin: test.server.local" --header "RandomHeader: Header value goes here"
http-check OK: HTTP Status 200 for http://localhost:8000/health
```

#### Note(s)

* When using `--redirect-ok` it affects both the string search and status checkfunctionality.
  - For a string search, if true, it searches for the string in the eventual destination. 
  - For a status check, if false, receiving a redirect will return a `warning` status.  If true, it will return an `ok` status.
* Headers should be in the form of "Header-Name: Header value".

### http-perf

#### Help output

```
HTTP Performance Check

Usage:
  http-perf [flags]
  http-perf [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -u, --url string               URL to test (default "http://localhost:80/")
  -T, --timeout int              Request timeout in seconds (default 15)
  -w, --warning string           Warning threshold, can be expressed as seconds or milliseconds (1s = 1000ms) (default "1s")
  -c, --critical string          Critical threshold, can be expressed as seconds or milliseconds (1s = 1000ms) (default "2s")
  -m, --output-in-ms             Provide output in milliseconds (default false, display in seconds)
  -H, --header strings           Additional header(s) to send in check request
  -C, --mtls-cert-file string    Certificate file for mutual TLS auth in PEM format
  -K, --mtls-key-file string     Key file for mutual TLS auth in PEM format
  -t, --trusted-ca-file string   TLS CA certificate bundle in PEM format
  -i, --insecure-skip-verify     Skip TLS certificate verification (not recommended!)
  -h, --help                     help for http-perf

Use "http-perf [command] --help" for more information about a command.
```

#### Example(s)

```
http-perf --url https://sensu.io --warning 1s --critical 2s
http-perf OK: 0.243321s | dns_duration=0.016596, tls_handshake_duration=0.172235, connect_duration=0.022199, first_byte_duration=0.243267, total_request_duration=0.243321

# Let's make the warning threshold pretty improbable
http-perf --url https://sensu.io --warning 10ms --critical 1s
http-perf WARNING: 0.304795s | dns_duration=0.033904, tls_handshake_duration=0.198843, connect_duration=0.021858, first_byte_duration=0.304747, total_request_duration=0.304795

# Again, but with output in milliseconds
http-perf --url https://sensu.io --warning 10ms --critical 1s --output-in-ms
http-perf WARNING: 262ms | dns_duration=35, tls_handshake_duration=170, connect_duration=22, first_byte_duration=262, total_request_duration=262

# With cusomter header(s)
http-perf --url https://sensu.io --warning 1s --critical 2s --header "Custom-Header: Custom header value"
http-perf OK: 0.243321s | dns_duration=0.016596, tls_handshake_duration=0.172235, connect_duration=0.022199, first_byte_duration=0.243267, total_request_duration=0.243321
```

#### Note(s)

* http-perf does **not** follow redirects, the page you are testing will need to
be referenced explicitly.
* Headers should be in the form of "Header-Name: Header value".

### http-json

#### Help output

```
HTTP JSON Check

Usage:
  http-json [flags]
  http-json [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -u, --url string               URL to test (default "http://localhost:80/")
  -q, --query string             Query written in jq format
  -e, --expression string        Expression for comparing result of query
  -H, --header strings           Additional header(s) to send in check request
  -C, --mtls-cert-file string    Certificate file for mutual TLS auth in PEM format
  -K, --mtls-key-file string     Key file for mutual TLS auth in PEM format
  -i, --insecure-skip-verify     Skip TLS certificate verification (not recommended!)
  -t, --trusted-ca-file string   TLS CA certificate bundle in PEM format
  -T, --timeout int              Request timeout in seconds (default 15)
  -h, --help                     help for http-json

Use "http-json [command] --help" for more information about a command.
```

#### Writing queries and expressions

Queries are written in [jq][6] format as implemented by the [gojq][7] library.
The query is used to obtain a value in the JSON requested from the URL
specified by `--url`. This value is then evaluated against the expression
provided by `--expression`.

#### Example(s)

```
# Boolean example - checking Sensu cluster health
http-json --url http://backend:8080/health --query ".ClusterHealth.[0].Healthy" --expression "== true"
http-json OK:  The value true found at .ClusterHealth.[0].Healthy matched with expression "== true" and returned true

# String comparison expressions
http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --query .id --expression "== \"HeaFdiyIJe\""
http-json OK:  The value HeaFdiyIJe found at .id matched with expression "== \"HeaFdiyIJe\"" and returned true

http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --query .id --expression "== \"BadText\""
http-json CRITICAL: The value HeaFdiyIJe found at .id did not match with expression "== \"BadText\"" and returned false

# Numeric comparison expressions
http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --query .status --expression "== 200"
http-json OK:  The value 200 found at .status matched with expression "== 200" and returned true

http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --query .status --expression "< 300"
http-json OK:  The value 200 found at .status matched with expression "< 300" and returned true

http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --query .status --expression "> 300"
http-json CRITICAL: The value 200 found at .status did not match with expression "> 300" and returned false

# With a custom header
http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --query .status --expression "< 300" --header "Custom-Header: Custom header value"
http-json OK:  The value 200 found at .status matched with expression "< 300" and returned true

```

#### Note(s)

* Headers should be in the form of "Header-Name: Header value".


### http-get

#### Help output

```
HTTP GET Check

Usage:
  http-get [flags]
  http-get [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -H, --header strings           Additional header(s) to send in check request
  -h, --help                     help for http-get
  -i, --insecure-skip-verify     Skip TLS certificate verification (not recommended!)
  -C, --mtls-cert-file string    Certificate file for mutual TLS auth in PEM format
  -K, --mtls-key-file string     Key file for mutual TLS auth in PEM format
  -T, --timeout int              Request timeout in seconds (default 15)
  -t, --trusted-ca-file string   TLS CA certificate bundle in PEM format
  -u, --url string               URL to get (default "http://localhost:80/")

Use "http-get [command] --help" for more information about a command.
```

#### Using the `http-get` Check

This check is intended to provide a Sensu-native way of fetching the output
of a HTTP GET request against a metrics endpoint, so that those metrics 
can be processed by Sensu's `output_metrics_threshold` features.

While this functionality is overlapped by default tools like `wget`, `curl`,
and Windows's `Invoke-WebRequest` PowerShell commandlet, this provides a 
cross-platform solution to fetching these HTTP-accessible metrics in a 
Sensu-native context.

This plugin will result in a critical exit status code if the body is empty
or if the HTTP GET request fails.

#### Example(s)

```
# Prometheus node_exporter example - fetching Prometheus metrics from node_exporter
http-get --url http://localhost:9100
[... output is the metrics in Prometheus format ...]
```

#### Note(s)

* Headers should be in the form of "Header-Name: Header value".


## Configuration

### Asset registration

[Sensu Assets][2] are the best way to make use of this plugin. If you're not
using an asset, please consider doing so! If you're using sensuctl 5.13 with
Sensu Backend 5.13 or later, you can use the following command to add the asset:

```
sensuctl asset add sensu/http-checks
```

If you're using an earlier version of sensuctl, you can find the asset on the [Bonsai Asset Index][3].

### Check definitions

#### http-check

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: http-check
  namespace: default
spec:
  command: http-check --url http://example.com
  subscriptions:
  - system
  runtime_assets:
  - sensu/http-checks
```

#### http-perf

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: http-perf
  namespace: default
spec:
  command: http-perf --url http://example.com
  subscriptions:
  - system
  runtime_assets:
  - sensu/http-checks
  output_metric_format: nagios_perfdata
  output_metric-handlers:
  - influxdb
```

#### http-json

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: http-json
  namespace: default
spec:
  command: http-json --url https://icanhazdadjoke.com/j/HeaFdiyIJe --path id --expression "== \"HeaFdiyIJe\""
  subscriptions:
  - system
  runtime_assets:
  - sensu/http-checks
```
#### http-get

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: prometheus-node-exporter
  namespace: default
spec:
  command: http-get --url http://localhost:9100
  subscriptions:
  - system
  runtime_assets:
  - sensu/http-checks:0.6.0
  output_metrics_format: prometheus_text
  output_metric_handlers:
  - metrics
  output_metric_tags:
  - name: entity
    value: "{{ .name }}"
```

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an
Asset. If you would like to compile and install the plugin from source or
contribute to it, download the latest version or create an executable script
from this source.

From the local path of the http-checks repository:

```
go build -o bin/http-check ./cmd/http-check
go build -o bin/http-perf ./cmd/http-perf
go build -o bin/http-json ./cmd/http-json
go build -o bin/http-get ./cmd/http-get
```

## Contributing

For more information about contributing to this plugin, see [Contributing][4].

[1]: https://docs.sensu.io/sensu-go/latest/reference/checks/
[2]: https://docs.sensu.io/sensu-go/latest/reference/assets/
[3]: https://bonsai.sensu.io/assets/sensu/http-checks
[4]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md
[5]: https://github.com/ncr-devops-platform/nagiosfoundation
[6]: https://github.com/stedolan/jq
[7]: https://github.com/itchyny/gojq
