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

	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/thumper/config"
	"github.com/levenlabs/thumper/context"
	"github.com/mitchellh/mapstructure"
)

// Actioner describes an action type. There all multiple action types, but they
// all simply attempt to perform one action and that's it
type Actioner interface {

	// Do takes in the alert context, and possibly returnes an error if the
	// action failed
	Do(context.Context) error
}

// Action is a wrapper around an Actioner which contains some type information
type Action struct {
	Type string
	Actioner
}

// ToActioner takes in some arbitrary data (hopefully a map[string]interface{},
// looks at its "type" key, and any other fields necessary based on that type,
// and returns an Actioner (or an error)
func ToActioner(in interface{}) (Action, error) {
	min, ok := in.(map[string]interface{})
	if !ok {
		return Action{}, errors.New("action definition is not an object")
	}

	var a Actioner
	typ, _ := min["type"].(string)
	typ = strings.ToLower(typ)
	switch typ {
	case "log":
		a = &Log{}
	case "http":
		a = &HTTP{}
	case "pagerduty":
		a = &PagerDuty{}
	default:
		return Action{}, fmt.Errorf("unknown action type: %q", typ)
	}

	if err := mapstructure.Decode(min, a); err != nil {
		return Action{}, err
	}
	return Action{Type: typ, Actioner: a}, nil
}

// Log is an action which does nothing but print a log message. Useful when
// testing alerts and you don't want to set up any actions yet
type Log struct {
	Message string `mapstructure:"message"`
}

// Do logs the Log's message. It doesn't actually need any context
func (l *Log) Do(_ context.Context) error {
	llog.Info("doing log action", llog.KV{"message": l.Message})
	return nil
}

// HTTP is an action which performs a single http request. If the request's
// response doesn't have a 2xx response code then it's considered an error
type HTTP struct {
	Method  string            `mapstructure:"method"`
	URL     string            `mapstructure:"url"`
	Headers map[string]string `mapstructure:"headers"`
	Body    string            `mapstructure:"body"`
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
	Key         string                 `mapstructure:"incident_key"`
	Description string                 `mapstructure:"description"`
	Details     map[string]interface{} `mapstructure:"details"`
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

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
