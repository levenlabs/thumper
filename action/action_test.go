package action

import (
	"net/http"
	"net/http/httptest"
	. "testing"

	"github.com/levenlabs/thumper/context"

	"gopkg.in/yaml.v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLUnmarshal(t *T) {
	j := []byte(`
type: http
method: get
url: http://example.com
body: wat`)
	a := Action{}
	require.Nil(t, yaml.Unmarshal(j, &a))
	assert.Equal(t, &HTTP{Method: "get", URL: "http://example.com", Body: "wat"}, a.Actioner)

	j = []byte(`
type: pagerduty
incident_key: foo
description: bar`)
	a = Action{}
	require.Nil(t, yaml.Unmarshal(j, &a))
	assert.Equal(t, &PagerDuty{Key: "foo", Description: "bar"}, a.Actioner)

	j = []byte(`
type: lua
lua_file: foo
lua_inline: bar`)
	a = Action{}
	require.Nil(t, yaml.Unmarshal(j, &a))
	assert.Equal(t, &Lua{File: "foo", Inline: "bar"}, a.Actioner)
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
