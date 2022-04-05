package main

import (
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

// TestExecuteCheck mega test as each case mutates global state
func TestExecuteCheck(t *testing.T) {

	testCasesStringSearch := []struct {
		status int
		search string
	}{
		{sensu.CheckStateOK, "SUCCESS"},
		{sensu.CheckStateCritical, "FAILURE"},
	}

	for _, tc := range testCasesStringSearch {
		t.Run("StringSerach: "+tc.search, func(t *testing.T) {
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
			defer test.Close()
			_, err := url.ParseRequestURI(test.URL)
			require.NoError(t, err)
			plugin.URL = test.URL
			plugin.SearchString = tc.search
			status, err := executeCheck(event)
			assert.NoError(err)
			assert.Equal(tc.status, status)
		})
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
		t.Run("StatusCodes", func(t *testing.T) {
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
			defer test.Close()
			_, err := url.ParseRequestURI(test.URL)
			require.NoError(t, err)
			plugin.URL = test.URL
			plugin.SearchString = ""
			plugin.RedirectOK = tc.allowRedirect
			status, err := executeCheck(event)
			assert.NoError(err)
			assert.Equal(tc.returnStatus, status)
		})
	}

	t.Run("HeaderValue", func(t *testing.T) {
		event := corev2.FixtureEvent("entity1", "check")
		assert := assert.New(t)

		var test = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal("Test Header 1 Value", r.Header.Get("Test-Header-1"))
			assert.Equal("Test Header 2 Value", r.Header.Get("Test-Header-2"))
		}))
		defer test.Close()
		_, err := url.ParseRequestURI(test.URL)
		require.NoError(t, err)
		plugin.URL = test.URL
		plugin.SearchString = ""
		plugin.Headers = []string{"Test-Header-1: Test Header 1 Value", "Test-Header-2: Test Header 2 Value"}
		status, err := executeCheck(event)
		assert.NoError(err)
		assert.Equal(sensu.CheckStateOK, status)
	})

	t.Run("Check Response Code", func(t *testing.T) {
		testSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/redirect/":
				http.Redirect(w, r, "/", http.StatusMovedPermanently)
			case "/forbidden/":
				w.WriteHeader(http.StatusForbidden)
			case "/error/":
				w.WriteHeader(http.StatusInternalServerError)
			default:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("<h1>Success</h1>"))
			}
		}))
		defer testSrv.Close()

		type Options struct {
			FollowRedirects bool
			SearchString    string
		}

		testCases := []struct {
			Name             string
			URL              string
			Options          Options
			ResponseCode     int
			ExpectedExitCode int
		}{
			{
				Name:             "Expect OK",
				URL:              "/",
				ResponseCode:     200,
				ExpectedExitCode: sensu.CheckStateOK,
			}, {
				Name:             "Expect OK, got forbidden",
				URL:              "/forbidden/",
				ResponseCode:     200,
				ExpectedExitCode: sensu.CheckStateCritical,
			}, {
				Name:             "Expect 301",
				URL:              "/redirect/",
				ResponseCode:     301,
				ExpectedExitCode: sensu.CheckStateOK,
			}, {
				Name:             "Expect 301 With Follow Redirect, got OK",
				URL:              "/redirect/",
				ResponseCode:     301,
				Options:          Options{FollowRedirects: true},
				ExpectedExitCode: sensu.CheckStateCritical,
			}, {
				Name:             "Expect OK, got forbidden",
				URL:              "/forbidden/",
				ResponseCode:     200,
				ExpectedExitCode: sensu.CheckStateCritical,
			}, {
				Name:             "Expect Accepted, got OK",
				URL:              "/",
				ResponseCode:     201,
				ExpectedExitCode: sensu.CheckStateCritical,
			}, {
				Name:             "Expect OK with Search String",
				URL:              "/",
				ResponseCode:     200,
				Options:          Options{SearchString: "Success"},
				ExpectedExitCode: sensu.CheckStateOK,
			}, {
				Name:             "Expect 500 with Search String, got OK",
				URL:              "/",
				ResponseCode:     500,
				Options:          Options{SearchString: "Success"},
				ExpectedExitCode: sensu.CheckStateCritical,
			}, {
				Name:             "Expect OK with Missing Search String, get OK",
				URL:              "/",
				ResponseCode:     200,
				Options:          Options{SearchString: "Fizz"},
				ExpectedExitCode: sensu.CheckStateCritical,
			},
		}
		event := corev2.FixtureEvent("entity1", "check")

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				plugin.Headers = []string{}
				plugin.InsecureSkipVerify = true
				plugin.RedirectOK = tc.Options.FollowRedirects
				plugin.ResponseCode = tc.ResponseCode
				plugin.SearchString = tc.Options.SearchString
				plugin.URL = testSrv.URL + tc.URL
				actualExitCode, _ := executeCheck(event)
				assert.Equal(t, tc.ExpectedExitCode, actualExitCode)
			})
		}

	})
}
