package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/thumper/config"
)

func main() {
	if config.AlertFileDir == "" {
		llog.Fatal("--alerts must be set")
	}

	fstat, err := os.Stat(config.AlertFileDir)
	if err != nil {
		llog.Fatal("failed getting alert definitions", llog.KV{"err": err})
	}

	files := make([]string, 0, 10)
	if !fstat.IsDir() {
		files = append(files, config.AlertFileDir)
	} else {
		fileInfos, err := ioutil.ReadDir(config.AlertFileDir)
		if err != nil {
			llog.Fatal("failed getting alert dir info", llog.KV{"err": err})
		}
		for _, fi := range fileInfos {
			if !fi.IsDir() {
				files = append(files, filepath.Join(config.AlertFileDir, fi.Name()))
			}
		}
	}

	for _, file := range files {
		kv := llog.KV{"file": file}
		var alerts []Alert
		b, err := ioutil.ReadFile(file)
		if err != nil {
			kv["err"] = err
			llog.Fatal("failed to read alert config", kv)
		}

		if err := yaml.Unmarshal(b, &alerts); err != nil {
			kv["err"] = err
			llog.Fatal("failed to parse yaml", kv)
		}

		for i := range alerts {
			kv["name"] = alerts[i].Name
			llog.Info("initializing alert", kv)
			if err := alerts[i].Init(); err != nil {
				kv["err"] = err
				llog.Fatal("failed to initialize alert", kv)
			}

			go alertSpin(alerts[i])
		}
	}

	select {}
}

func alertSpin(a Alert) {
	for {
		now := time.Now()
		next := a.cron.Next(now)
		time.Sleep(next.Sub(now))
		go a.Run()
	}
}
