package search

import (
	. "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestDict(t *T) {
	s := []byte(`
foo: 1
bar:
  baz: buz
  box: wat
biz:
  - something
  - a: 1
    b: 2
  - c: 3
    d: 4`)
	d := Dict{}
	require.Nil(t, yaml.Unmarshal(s, &d))
	assert.Equal(t, 1, d["foo"])
	assert.Equal(t, Dict{"baz": "buz", "box": "wat"}, d["bar"])
	assert.Equal(t, "something", d["biz"].([]interface{})[0])
	assert.Equal(t, Dict{"a": 1, "b": 2}, d["biz"].([]interface{})[1])
	assert.Equal(t, Dict{"c": 3, "d": 4}, d["biz"].([]interface{})[2])
}
