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
	Process     luautil.LuaRunner `yaml:"process"`

	cron                                     *cronexpr.Expression
	searchIndexTPL, searchTypeTPL, searchTPL *template.Template
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

	llog.Debug("running process step", kv)
	processRes, ok := a.Process.Do(c)
	if !ok {
		llog.Error("failed at process step", kv)
		return
	}

	// if processRes isn't an []interface{}, actionsRaw will be the nil value of
	// []interface{}, which has a length of 0, so either way this works
	actionsRaw, _ := processRes.([]interface{})
	if len(actionsRaw) == 0 {
		llog.Debug("no actions returned", kv)
	}

	actions := make([]action.Action, len(actionsRaw))
	for i := range actionsRaw {
		a, err := action.ToActioner(actionsRaw[i])
		if err != nil {
			kv["err"] = err
			llog.Error("error unpacking action", kv)
			return
		}
		actions[i] = a
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
