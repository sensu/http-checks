package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
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
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("SUCCESS"))
	}))
	_, err := url.ParseRequestURI(test.URL)
	require.NoError(t, err)
	plugin.URL = test.URL
	warning, _ = time.ParseDuration("2s")
	critical, _ = time.ParseDuration("5s")
	status, err := executeCheck(event)
	assert.NoError(err)
	assert.Equal(sensu.CheckStateOK, status)
}
