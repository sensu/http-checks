package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
}

func TestExecuteCheck(t *testing.T) {

	event := corev2.FixtureEvent("entity1", "check")
	assert := assert.New(t)

	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedMethod := "GET"
		expectedURI := "/"
		assert.Equal(expectedMethod, r.Method)
		assert.Equal(expectedURI, r.RequestURI)
		assert.Equal("Test Header 1 Value", r.Header.Get("Test-Header-1"))
		assert.Equal("Test Header 2 Value", r.Header.Get("Test-Header-2"))
		assert.Equal("foo.bar.tld", r.Host)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("SUCCESS"))
	}))
	_, err := url.ParseRequestURI(test.URL)
	require.NoError(t, err)
	plugin.URL = test.URL
	plugin.Headers = []string{"Test-Header-1: Test Header 1 Value", "Test-Header-2: Test Header 2 Value", "Host: foo.bar.tld"}
	warning, _ = time.ParseDuration("2s")
	critical, _ = time.ParseDuration("5s")
	status, err := executeCheck(event)
	assert.NoError(err)
	assert.Equal(sensu.CheckStateOK, status)
}
