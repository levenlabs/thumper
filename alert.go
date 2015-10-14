package main

import (
	"bytes"
	"text/template"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/thumper/action"
	"github.com/levenlabs/thumper/context"
	"github.com/levenlabs/thumper/luautil"
	"github.com/levenlabs/thumper/search"
)

// Alert encompasses a search query which will be run periodically, the results
// of which will be checked against a condition. If the condition returns true a
// set of actions will be performed
type Alert struct {
	Name      string            `yaml:"name"`
	Interval  string            `yaml:"interval"`
	Search    interface{}       `yaml:"search"`
	Condition luautil.LuaRunner `yaml:"condition"`
	Actions   []yaml.MapSlice   `yaml:"actions"` // we keep the raw form for later templating

	tpls []*template.Template
}

// Init initializes some internal data inside the Alert, and must be called
// after the Alert is unmarshaled from yaml (or otherwise created)
func (a *Alert) Init() error {
	a.tpls = make([]*template.Template, len(a.Actions))
	for i := range a.Actions {
		b, err := yaml.Marshal(&a.Actions[i])
		if err != nil {
			return err
		}

		tpl, err := template.New("").Parse(string(b))
		if err != nil {
			return err
		}
		a.tpls[i] = tpl
	}
	return nil
}

func (a Alert) Run() {
	kv := llog.KV{
		"name": a.Name,
	}
	llog.Info("running alert", kv)

	if len(a.Actions) == 0 {
		llog.Warn("no actions defined, not even going to bother running", kv)
		return
	}

	c := context.Context{
		Name:      a.Name,
		StartedTS: uint64(time.Now().Unix()),
	}

	// TODO need to be able to specify index and type in the config
	res, err := search.Search("_all", "_all", a.Search)
	if err != nil {
		kv["err"] = err
		llog.Error("failed at search step", kv)
		return
	}
	c.Result = res

	doActions, ok := a.Condition.Do(c)
	if !ok {
		llog.Error("failed at condition step", kv)
		return
	} else if !doActions {
		llog.Debug("doActions returned false", kv)
		return
	}

	actions, err := a.createActions(c)
	if err != nil {
		kv["err"] = err
		llog.Error("failed to create action data", kv)
		return
	}

	for i := range actions {
		kv["action"] = actions[i].Type
		llog.Info("performing action", kv)
		if err := actions[i].Do(c); err != nil {
			kv["err"] = err
			llog.Error("failed to complete action", kv)
			return
		}
	}
}

func (a Alert) createActions(c context.Context) ([]action.Action, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	actions := make([]action.Action, len(a.tpls))

	for i := range a.tpls {
		buf.Reset()
		if err := a.tpls[i].Execute(buf, &c); err != nil {
			return nil, err
		}

		b := buf.Bytes()
		if err := yaml.Unmarshal(b, &actions[i]); err != nil {
			return nil, err
		}
	}

	return actions, nil
}
