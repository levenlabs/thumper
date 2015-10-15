package luautil

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	. "testing"

	"github.com/Shopify/go-lua"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLuaState() *lua.State {
	l := lua.NewState()
	lua.OpenLibraries(l)
	return l
}

func testPushFrom(t *T, f func(*lua.State, reflect.Value), i interface{}, code string) {
	l := testLuaState()
	initialStackSize := l.Top()

	f(l, reflect.ValueOf(i))
	l.SetGlobal("ctx")
	assert.Equal(t, initialStackSize, l.Top())

	b := bytes.NewBufferString(code)
	require.Nil(t, l.Load(b, "", "bt"))
	l.Call(0, 1)
	assert.True(t, l.ToBoolean(-1))
	l.Remove(-1)
	assert.Equal(t, initialStackSize, l.Top())
}

func TestTableFromStruct(t *T) {

	type Foo struct {
		A int
		B string
	}

	type Bar struct {
		C Foo
		D bool `luautil:"d"`
	}

	type Baz struct {
		Bar `luautil:",inline"`
		E   string
		F   int `luautil:"-"`
	}

	i := Baz{Bar{Foo{1, "wat"}, true}, "wut", 5}
	testPushFrom(t, pushTableFromStruct, i, `
		if ctx.C.A ~= 1 then return false end
		if ctx.C.B ~= "wat" then return false end
		if ctx.d ~= true then return false end
		if ctx.E ~= "wut" then return false end
		if ctx.F ~= nil then return false end
		return true
	`)
}

func TestTableFromMap(t *T) {
	m := map[interface{}]interface{}{
		"A": 1,
		5:   "FOO",
		true: map[string]interface{}{
			"foo": "bar",
		},
	}
	testPushFrom(t, pushTableFromMap, m, `
		if ctx.A ~= 1 then return false end
		if ctx[5] ~= "FOO" then return false end
		if ctx[true].foo ~= "bar" then return false end
		return true
	`)
}

func TestTableFromSlice(t *T) {
	s := []interface{}{
		"foo",
		true,
		4,
		[]string{
			"bar",
			"baz",
		},
	}
	testPushFrom(t, pushTableFromSlice, s, `
		if ctx[1] ~= "foo" then return false end
		if ctx[2] ~= true then return false end
		if ctx[3] ~= 4 then return false end
		if ctx[4][1] ~= "bar" then return false end
		if ctx[4][2] ~= "baz" then return false end
		return true
	`)
}

func TestRun(t *T) {
	ctx := map[string]interface{}{
		"foo": "foo",
	}
	code := `return ctx.foo == "foo"`

	ret, ok := RunInline(ctx, code)
	assert.True(t, ok)
	assert.True(t, ret)

	ctx["foo"] = false
	ret, ok = RunInline(ctx, code)
	assert.True(t, ok)
	assert.False(t, ret)

	f, err := ioutil.TempFile("", "")
	require.Nil(t, err)
	filename := f.Name()
	defer os.Remove(filename)
	_, err = io.WriteString(f, code)
	require.Nil(t, err)
	f.Close()

	ctx["foo"] = "foo"
	ret, ok = RunFile(ctx, filename)
	assert.True(t, ok)
	assert.True(t, ret)

	ctx["foo"] = false
	ret, ok = RunFile(ctx, filename)
	assert.True(t, ok)
	assert.False(t, ret)
}
