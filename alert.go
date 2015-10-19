package main

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/gorhill/cronexpr"
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
	Name        string            `yaml:"name"`
	Interval    string            `yaml:"interval"`
	SearchIndex string            `yaml:"search_index"`
	SearchType  string            `yaml:"search_type"`
	Search      search.Dict       `yaml:"search"`
	Condition   luautil.LuaRunner `yaml:"condition"`
	Actions     []interface{}     `yaml:"actions"`

	cron                                     *cronexpr.Expression
	searchIndexTPL, searchTypeTPL, searchTPL *template.Template
	actionTPLs                               []*template.Template
}

func templatizeHelper(i interface{}, lastErr error) (*template.Template, error) {
	if lastErr != nil {
		return nil, lastErr
	}
	var str string
	if s, ok := i.(string); ok {
		str = s
	} else {
		b, err := yaml.Marshal(i)
		if err != nil {
			return nil, err
		}
		str = string(b)
	}

	return template.New("").Parse(str)
}

// Init initializes some internal data inside the Alert, and must be called
// after the Alert is unmarshaled from yaml (or otherwise created)
func (a *Alert) Init() error {
	var err error
	a.searchIndexTPL, err = templatizeHelper(a.SearchIndex, err)
	a.searchTypeTPL, err = templatizeHelper(a.SearchType, err)
	a.searchTPL, err = templatizeHelper(&a.Search, err)

	a.actionTPLs = make([]*template.Template, len(a.Actions))
	for i := range a.Actions {
		a.actionTPLs[i], err = templatizeHelper(&a.Actions[i], err)
	}
	if err != nil {
		return err
	}

	cron, err := cronexpr.Parse(a.Interval)
	if err != nil {
		return fmt.Errorf("parsing interval: %s", err)
	}
	a.cron = cron

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

	now := time.Now()
	c := context.Context{
		Name:      a.Name,
		StartedTS: uint64(now.Unix()),
		Time:      now,
	}

	searchIndex, searchType, searchQuery, err := a.createSearch(c)
	if err != nil {
		kv["err"] = err
		llog.Error("failed to create search data", kv)
		return
	}

	llog.Debug("running search step", kv)
	res, err := search.Search(searchIndex, searchType, searchQuery)
	if err != nil {
		kv["err"] = err
		llog.Error("failed at search step", kv)
		return
	}
	c.Result = res

	llog.Debug("running condition step", kv)
	doActions, ok := a.Condition.Do(c)
	if !ok {
		llog.Error("failed at condition step", kv)
		return
	} else if !doActions {
		llog.Debug("doActions is false", kv)
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

func (a Alert) createSearch(c context.Context) (string, string, interface{}, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	if err := a.searchIndexTPL.Execute(buf, &c); err != nil {
		return "", "", nil, err
	}
	searchIndex := buf.String()

	buf.Reset()
	if err := a.searchTypeTPL.Execute(buf, &c); err != nil {
		return "", "", nil, err
	}
	searchType := buf.String()

	buf.Reset()
	if err := a.searchTPL.Execute(buf, &c); err != nil {
		return "", "", nil, err
	}
	searchRaw := buf.Bytes()

	var search search.Dict
	if err := yaml.Unmarshal(searchRaw, &search); err != nil {
		return "", "", nil, err
	}

	return searchIndex, searchType, search, nil
}

func (a Alert) createActions(c context.Context) ([]action.Action, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	actions := make([]action.Action, len(a.actionTPLs))

	for i := range a.actionTPLs {
		buf.Reset()
		if err := a.actionTPLs[i].Execute(buf, &c); err != nil {
			return nil, err
		}

		b := buf.Bytes()
		if err := yaml.Unmarshal(b, &actions[i]); err != nil {
			return nil, err
		}
	}

	return actions, nil
}
