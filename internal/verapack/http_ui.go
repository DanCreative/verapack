//go:build ui

package verapack

import (
	"crypto/tls"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/DanCreative/veracode-go/veracode"
)

type apiCredentials_ui struct {
	ApiId        string `json:"api_id"`
	ApiSecret    string `json:"api_secret"`
	ExpirationTs string `json:"expiration_ts"`
}

type sandbox_ui struct {
	ApplicationGuid string `json:"application_guid,omitempty"`
	Guid            string `json:"guid,omitempty"`
	Id              int    `json:"id,omitempty"`
	Name            string `json:"name,omitempty"`
}

type application struct {
	Guid    string  `json:"guid,omitempty"`
	Profile profile `json:"profile"`
}

type profile struct {
	Name string `json:"name,omitempty"`
}

func newVeracodeMockServer(config Config) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		time.Sleep(time.Duration(rand.IntN(500)+100) * time.Millisecond)

		if strings.HasSuffix(r.URL.Path, "/promote") {
			sandbox := sandbox_ui{
				Name: "Release Candidate",
			}

			out, _ := json.Marshal(sandbox)

			w.WriteHeader(200)
			w.Write(out)
			return
		}

		if r.URL.Path == "/appsec/v1/applications" {
			type applicationSearchResult struct {
				Embedded struct {
					Applications []application `json:"applications"`
				} `json:"_embedded"`
			}

			results := applicationSearchResult{
				Embedded: struct {
					Applications []application "json:\"applications\""
				}{
					Applications: make([]application, 0, len(config.Applications)),
				},
			}

			for k, app := range config.Applications {
				results.Embedded.Applications = append(results.Embedded.Applications, application{Guid: strconv.Itoa(k), Profile: profile{Name: app.AppName}})
			}

			out, _ := json.Marshal(&results)
			w.WriteHeader(200)
			w.Write(out)
			return
		}

		if r.URL.Path == "/api/authn/v2/api_credentials" {
			t := time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z0700")
			results := apiCredentials_ui{
				ExpirationTs: t,
				ApiId:        "vera-123",
				ApiSecret:    "vera-12345",
			}

			out, _ := json.Marshal(&results)
			w.WriteHeader(200)
			w.Write(out)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/sandboxes") && (r.Method == http.MethodGet || r.Method == "") {
			type sandboxSearchResult struct {
				Embedded struct {
					Sandboxes []sandbox_ui `json:"sandboxes"`
				} `json:"_embedded"`
			}

			results := sandboxSearchResult{
				Embedded: struct {
					Sandboxes []sandbox_ui "json:\"sandboxes\""
				}{
					Sandboxes: make([]sandbox_ui, 0, len(config.Applications)),
				},
			}

			for k, app := range config.Applications {
				results.Embedded.Sandboxes = append(results.Embedded.Sandboxes, sandbox_ui{
					Name:            app.SandboxName,
					Id:              1,
					Guid:            "1",
					ApplicationGuid: strconv.Itoa(k),
				})
			}

			out, err := json.Marshal(&results)
			if err != nil {
				w.WriteHeader(500)
				w.Write([]byte(err.Error()))
				return
			}

			w.WriteHeader(200)
			w.Write(out)
			return
		}

		w.WriteHeader(500)

		type e struct {
			Message string `json:"message"`
		}

		err := e{
			Message: r.Method,
		}

		out, _ := json.Marshal(&err)
		w.Write(out)
	}))
}

func newVeracodeMockClient(mockServerHost string) (*veracode.Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{
		Transport: &mockRoundtripper{
			mockServerHost: mockServerHost,
			transport:      tr,
		},
	}

	client, err := veracode.NewClient(httpClient, "", "")
	if err != nil {
		return nil, err
	}

	return client, nil
}

type mockRoundtripper struct {
	mockServerHost string
	transport      http.RoundTripper
}

func (m *mockRoundtripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Host = m.mockServerHost

	return m.transport.RoundTrip(req)
}
