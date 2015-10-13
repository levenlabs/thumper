package luautil

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"

	"github.com/Shopify/go-lua"
	"github.com/levenlabs/go-llog"
)

type cmd struct {
	ctx              interface{}
	filename, inline string
	retCh            chan bool
}

var cmdCh = make(chan cmd)

// RunInline takes the given lua code, and runs it with the given ctx variable
// set as the lua global variable "ctx". The lua code is expected to return a
// boolean value, which is passed back as the first boolean return. The second
// boolean return will be false if there was an error and the code wasn't run
func RunInline(ctx interface{}, code string) (bool, bool) {
	c := cmd{
		ctx:    ctx,
		inline: code,
		retCh:  make(chan bool),
	}
	cmdCh <- c
	ret, ok := <-c.retCh
	return ret, ok
}

// RunFile is similar to RunInline, except it takes in a filename which has the
// lua code to run. Note that the file's contents are cached, so the file is
// only opened and read the first time it's used.
func RunFile(ctx interface{}, filename string) (bool, bool) {
	c := cmd{
		ctx:      ctx,
		filename: filename,
		retCh:    make(chan bool),
	}
	cmdCh <- c
	ret, ok := <-c.retCh
	return ret, ok
}

type runner struct {
	l *lua.State

	// Set of files and inline functions already in the global namespace
	m map[string]bool
}

func init() {
	newRunner()
}

func newRunner() {
	l := lua.NewState()
	lua.OpenLibraries(l)
	r := runner{
		l: l,
		m: map[string]bool{},
	}
	go r.spin()
}

func (r *runner) spin() {
	for c := range cmdCh {
		var fnName string
		var err error
		kv := llog.KV{}
		if c.filename != "" {
			kv["filename"] = c.filename
			fnName, err = r.loadFile(c.filename)
		} else {
			kv["inline"] = c.inline[:20]
			fnName, err = r.loadInline(c.inline)
		}
		if err != nil {
			kv["err"] = err
			llog.Error("error loading lua", kv)
			close(c.retCh)
			continue
		}

		kv["fnName"] = fnName
		llog.Info("loaded lua function", kv)

		pushArbitraryValue(r.l, c.ctx) // push ctx onto the stack
		r.l.SetGlobal("ctx")           // set global variable "ctx" to ctx, pops it from stack
		r.l.Global(fnName)             // push function onto stack
		r.l.Call(0, 1)                 // call function, pops function from stack, pushes return
		c.retCh <- r.l.ToBoolean(-1)   // send back function return
		r.l.Remove(-1)                 // pop function return from stack
		// stack is now clean
	}
}

func (r *runner) loadFile(name string) (string, error) {
	key := quickSha(name)
	if r.m[key] {
		return key, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := r.l.Load(f, name, "bt"); err != nil {
		return "", err
	}
	r.l.SetGlobal(key)

	r.m[key] = true
	return key, nil
}

func (r *runner) loadInline(code string) (string, error) {
	key := quickSha(code)
	if r.m[key] {
		return key, nil
	}
	if err := r.l.Load(bytes.NewBufferString(code), key, "bt"); err != nil {
		return "", err
	}
	r.l.SetGlobal(key)

	r.m[key] = true
	return key, nil
}

func quickSha(s string) string {
	sh := sha1.New()
	sh.Write([]byte(s))
	return hex.EncodeToString(sh.Sum(nil))
}

func pushArbitraryValue(l *lua.State, i interface{}) {
	if i == nil {
		l.PushNil()
		return
	}

	switch ii := i.(type) {
	case bool:
		l.PushBoolean(ii)
	case int:
		l.PushInteger(ii)
	case int8:
		l.PushInteger(int(ii))
	case int16:
		l.PushInteger(int(ii))
	case int32:
		l.PushInteger(int(ii))
	case int64:
		l.PushInteger(int(ii))
	case uint:
		l.PushUnsigned(ii)
	case uint8:
		l.PushUnsigned(uint(ii))
	case uint16:
		l.PushUnsigned(uint(ii))
	case uint32:
		l.PushUnsigned(uint(ii))
	case uint64:
		l.PushUnsigned(uint(ii))
	case float64:
		l.PushNumber(ii)
	case float32:
		l.PushNumber(float64(ii))
	case string:
		l.PushString(ii)
	case []byte:
		l.PushString(string(ii))
	default:

		v := reflect.ValueOf(i)
		switch v.Kind() {
		case reflect.Ptr:
			pushArbitraryValue(l, v.Elem().Interface())

		case reflect.Struct:
			pushTableFromStruct(l, v)

		case reflect.Map:
			pushTableFromMap(l, v)

		case reflect.Slice:
			pushTableFromSlice(l, v)

		default:
			panic(fmt.Sprintf("unknown type being pushed onto lua stack: %T %+v", i, i))
		}

	}
}

func pushTableFromStruct(l *lua.State, v reflect.Value) {
	l.NewTable()
	t := v.Type()
	for j := 0; j < v.NumField(); j++ {
		pushArbitraryValue(l, t.Field(j).Name)
		pushArbitraryValue(l, v.Field(j).Interface())
		l.SetTable(-3)
	}
}

func pushTableFromMap(l *lua.State, v reflect.Value) {
	l.NewTable()
	for _, k := range v.MapKeys() {
		pushArbitraryValue(l, k.Interface())
		pushArbitraryValue(l, v.MapIndex(k).Interface())
		l.SetTable(-3)
	}
}

func pushTableFromSlice(l *lua.State, v reflect.Value) {
	l.NewTable()
	for j := 0; j < v.Len(); j++ {
		pushArbitraryValue(l, j+1) // because lua is 1-indexed
		pushArbitraryValue(l, v.Index(j).Interface())
		l.SetTable(-3)
	}
}
