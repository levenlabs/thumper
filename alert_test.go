package main

import (
	. "testing"

	"github.com/levenlabs/thumper/action"
	"github.com/levenlabs/thumper/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestActionTPL(t *T) {
	y := []byte(`
actions:
  - type: pagerduty
    description: "{{.Name}}"`)

	var a Alert
	require.Nil(t, yaml.Unmarshal(y, &a))
	require.Nil(t, a.Init())
	assert.NotEmpty(t, a.tpls)

	c := context.Context{
		Name: "foo",
	}
	actions, err := a.createActions(c)
	require.Nil(t, err)
	require.NotEmpty(t, actions)
	assert.Equal(t, actions[0].Actioner.(*action.PagerDuty).Description, "foo")
}
