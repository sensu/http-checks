package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
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
		responseCode  []string
	}{
		{sensu.CheckStateOK, http.StatusOK, false, nil},
		{sensu.CheckStateOK, http.StatusOK, true, nil},
		{sensu.CheckStateCritical, http.StatusNotFound, true, []string{"301"}},
		{sensu.CheckStateOK, http.StatusMovedPermanently, false, []string{"301"}},
		{sensu.CheckStateOK, http.StatusOK, false, []string{"200"}},
		{sensu.CheckStateOK, http.StatusNotFound, false, []string{"200", "404"}},
		{sensu.CheckStateOK, http.StatusCreated, false, []string{"200", "201"}},
		{sensu.CheckStateWarning, http.StatusMovedPermanently, false, nil},
		{sensu.CheckStateCritical, http.StatusBadRequest, false, nil},
		{sensu.CheckStateCritical, http.StatusInternalServerError, false, nil},
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
		plugin.ResponseCode = tc.responseCode
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
