package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
}

func TestExecuteCheck(t *testing.T) {

	type testData struct {
		Text   string `json:"text"`
		Number int    `json:"number"`
	}
	td := &testData{
		Text:   "testing",
		Number: 10,
	}

	testJSON, _ := json.Marshal(td)

	event := corev2.FixtureEvent("entity1", "check")

	testCases := []struct {
		status     int
		path       string
		expression string
	}{
		{sensu.CheckStateOK, "text", "== \"testing\""},
		{sensu.CheckStateCritical, "text", "== \"notfound\""},
		{sensu.CheckStateOK, "number", "== 10"},
		{sensu.CheckStateCritical, "number", "== 11"},
		{sensu.CheckStateOK, "number", ">= 10"},
		{sensu.CheckStateOK, "number", "> 9"},
		{sensu.CheckStateCritical, "number", ">= 11"},
		{sensu.CheckStateCritical, "number", "> 12"},
		{sensu.CheckStateOK, "number", "<= 10"},
		{sensu.CheckStateOK, "number", "< 11"},
		{sensu.CheckStateCritical, "number", "<= 9"},
		{sensu.CheckStateCritical, "number", "< 8"},
	}

	for _, tc := range testCases {
		assert := assert.New(t)

		var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expectedMethod := "GET"
			expectedURI := "/"
			assert.Equal(expectedMethod, r.Method)
			assert.Equal(expectedURI, r.RequestURI)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(testJSON)
		}))
		_, err := url.ParseRequestURI(test.URL)
		require.NoError(t, err)
		plugin.URL = test.URL
		plugin.Expression = tc.expression
		plugin.Path = tc.path
		status, err := executeCheck(event)
		assert.NoError(err)
		assert.Equal(tc.status, status)
	}
}
