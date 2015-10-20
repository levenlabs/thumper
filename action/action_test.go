package action

import (
	"net/http"
	"net/http/httptest"
	. "testing"

	"github.com/levenlabs/thumper/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToActioner(t *T) {
	m := map[string]interface{}{
		"type":   "http",
		"method": "get",
		"url":    "http://example.com",
		"body":   "wat",
	}
	a, err := ToActioner(m)
	assert.Nil(t, err)
	assert.Equal(t, &HTTP{Method: "get", URL: "http://example.com", Body: "wat"}, a.Actioner)

	m = map[string]interface{}{
		"type":         "pagerduty",
		"incident_key": "foo",
		"description":  "bar",
	}
	a, err = ToActioner(m)
	assert.Nil(t, err)
	assert.Equal(t, &PagerDuty{Key: "foo", Description: "bar"}, a.Actioner)

}

func TestHTTPAction(t *T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/good", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	})
	s := httptest.NewServer(mux)

	h := &HTTP{
		Method: "GET",
		URL:    s.URL + "/good",
		Body:   "OHAI",
	}
	require.Nil(t, h.Do(context.Context{}))

	h.URL = s.URL + "/bad"
	require.NotNil(t, h.Do(context.Context{}))
}
