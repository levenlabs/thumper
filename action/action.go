// Package action implements all the different actions an alert can take should
// its condition be found to be true
package action

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/levenlabs/thumper/config"
	"github.com/levenlabs/thumper/context"
	"github.com/levenlabs/thumper/luautil"
)

// Actioner describes an action type. There all multiple action types, but they
// all simply attempt to perform one action and that's it
type Actioner interface {

	// Do takes in the alert context, and possibly returnes an error if the
	// action failed
	Do(context.Context) error
}

// Action is a wrapper around an Actioner which can be yaml unmarshalled into
// easily
type Action struct {
	Type string
	Actioner
}

// UnmarshalYAML unmarshals the given yaml into the embedded Actioner, depending
// on the type field in the yaml
func (a *Action) UnmarshalYAML(unmarshal func(interface{}) error) error {
	at := struct {
		Type string `yaml:"type"`
	}{}

	if err := unmarshal(&at); err != nil {
		return err
	}
	a.Type = strings.ToLower(at.Type)

	switch a.Type {
	case "http":
		a.Actioner = &HTTP{}
	case "pagerduty":
		a.Actioner = &PagerDuty{}
	case "lua":
		a.Actioner = &Lua{}
	default:
		return fmt.Errorf("invalid action type %q", a.Type)
	}

	if err := unmarshal(a.Actioner); err != nil {
		return err
	}

	return nil
}

// HTTP is an action which performs a single http request. If the request's
// response doesn't have a 2xx response code then it's considered an error
type HTTP struct {
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

// Do performs the actual http request. It doesn't need the alert context
func (h *HTTP) Do(_ context.Context) error {
	r, err := http.NewRequest(h.Method, h.URL, bytes.NewBufferString(h.Body))
	if err != nil {
		return err
	}

	if h.Headers != nil {
		for k, v := range h.Headers {
			r.Header.Set(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("non 2xx response code returned: %d", resp.StatusCode)
	}

	return nil
}

// PagerDuty submits a trigger to a pagerduty endpoint
type PagerDuty struct {
	Key         string                 `yaml:"incident_key"`
	Description string                 `yaml:"description"`
	Details     map[string]interface{} `yaml:"details"`
}

// Do performs the actual trigger request to the pagerduty api
func (p *PagerDuty) Do(c context.Context) error {
	if config.PagerDutyKey == "" {
		return errors.New("pagerduty key not set in config")
	}
	if p.Key == "" {
		p.Key = c.Name
	}

	body := map[string]interface{}{
		"service_key":  config.PagerDutyKey,
		"event_type":   "trigger",
		"description":  p.Description,
		"incident_key": p.Key,
		"details":      p.Details,
	}
	bodyb, err := json.Marshal(&body)
	if err != nil {
		return err
	}

	u := "https://events.pagerduty.com/generic/2010-04-15/create_event.json"
	r, err := http.NewRequest("POST", u, bytes.NewBuffer(bodyb))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")

	_, err = http.DefaultClient.Do(r)
	return err
}

// Lua is a wrapper around a LuaRunner which implements the Actioner interface
type Lua struct {
	luautil.LuaRunner `yaml:",inline"`
}

// Do performs the lua action
func (l *Lua) Do(c context.Context) error {
	if _, ok := l.LuaRunner.Do(c); !ok {
		return errors.New("error running lua action")
	}
	return nil
}
