package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
}

func TestExecuteCheck(t *testing.T) {

	testCasesStringSearch := []struct {
		status int
		search string
	}{
		{sensu.CheckStateOK, "SUCCESS"},
		{sensu.CheckStateCritical, "FAILURE"},
	}

	for _, tc := range testCasesStringSearch {
		event := corev2.FixtureEvent("entity1", "check")
		assert := assert.New(t)

		var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expectedMethod := "GET"
			expectedURI := "/"
			assert.Equal(expectedMethod, r.Method)
			assert.Equal(expectedURI, r.RequestURI)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("SUCCESS"))
		}))
		_, err := url.ParseRequestURI(test.URL)
		require.NoError(t, err)
		plugin.URL = test.URL
		plugin.SearchString = tc.search
		status, err := executeCheck(event)
		assert.NoError(err)
		assert.Equal(tc.status, status)
	}

	testCasesStatus := []struct {
		returnStatus  int
		httpStatus    int
		allowRedirect bool
	}{
		{sensu.CheckStateOK, http.StatusOK, false},
		{sensu.CheckStateOK, http.StatusOK, true},
		{sensu.CheckStateWarning, http.StatusMovedPermanently, false},
		{sensu.CheckStateCritical, http.StatusBadRequest, false},
		{sensu.CheckStateCritical, http.StatusInternalServerError, false},
	}

	for _, tc := range testCasesStatus {
		event := corev2.FixtureEvent("entity1", "check")
		assert := assert.New(t)

		var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expectedMethod := "GET"
			expectedURI := "/"
			assert.Equal(expectedMethod, r.Method)
			assert.Equal(expectedURI, r.RequestURI)
			if tc.httpStatus >= http.StatusMultipleChoices && tc.httpStatus < http.StatusBadRequest {
				w.Header().Add("Location", "https://google.com")
			}
			w.WriteHeader(tc.httpStatus)
		}))
		_, err := url.ParseRequestURI(test.URL)
		require.NoError(t, err)
		plugin.URL = test.URL
		plugin.SearchString = ""
		plugin.RedirectOK = tc.allowRedirect
		status, err := executeCheck(event)
		assert.NoError(err)
		assert.Equal(tc.returnStatus, status)
	}

	event := corev2.FixtureEvent("entity1", "check")
	assert := assert.New(t)

	var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal("Test Header 1 Value", r.Header.Get("Test-Header-1"))
		assert.Equal("Test Header 2 Value", r.Header.Get("Test-Header-2"))
		assert.Equal("foo.bar.tld", r.Host)
	}))
	_, err := url.ParseRequestURI(test.URL)
	require.NoError(t, err)
	plugin.URL = test.URL
	plugin.SearchString = ""
	plugin.Headers = []string{"Test-Header-1: Test Header 1 Value", "Test-Header-2: Test Header 2 Value", "Host: foo.bar.tld"}
	status, err := executeCheck(event)
	assert.NoError(err)
	assert.Equal(sensu.CheckStateOK, status)
}
