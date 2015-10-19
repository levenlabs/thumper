package main

import (
	. "testing"

	"github.com/levenlabs/thumper/action"
	"github.com/levenlabs/thumper/context"
	"github.com/levenlabs/thumper/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestSearchTPL(t *T) {
	y := []byte(`
interval: "* * * * *"
search_index: foo-{{.Name}}
search_type: bar-{{.Name}}
search: {
	"query": {
		"query_string": {
			"query":"baz-{{.Name}}"
		}
	}
}`)

	var a Alert
	require.Nil(t, yaml.Unmarshal(y, &a))
	require.Nil(t, a.Init())
	require.NotNil(t, a.searchIndexTPL)
	require.NotNil(t, a.searchTypeTPL)
	require.NotNil(t, a.searchTPL)

	c := context.Context{
		Name: "wat",
	}
	searchIndex, searchType, searchQuery, err := a.createSearch(c)
	require.Nil(t, err)
	assert.Equal(t, "foo-wat", searchIndex)
	assert.Equal(t, "bar-wat", searchType)
	expectedSearch := search.Dict{
		"query": search.Dict{
			"query_string": search.Dict{
				"query": "baz-wat",
			},
		},
	}
	assert.Equal(t, expectedSearch, searchQuery)
}

func TestActionTPL(t *T) {
	y := []byte(`
interval: "* * * * *"
actions:
  - type: pagerduty
    description: "{{.Name}}"`)

	var a Alert
	require.Nil(t, yaml.Unmarshal(y, &a))
	require.Nil(t, a.Init())
	assert.NotEmpty(t, a.actionTPLs)

	c := context.Context{
		Name: "foo",
	}
	actions, err := a.createActions(c)
	require.Nil(t, err)
	require.NotEmpty(t, actions)
	assert.Equal(t, actions[0].Actioner.(*action.PagerDuty).Description, "foo")
}
