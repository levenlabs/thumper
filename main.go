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

			if config.ForceRun != "" && config.ForceRun == alerts[i].Name {
				alerts[i].Run()
				time.Sleep(250 * time.Millisecond) // allow time for logs to print
				return
			} else if config.ForceRun == "" {
				go alertSpin(alerts[i])
			}
		}
	}

	// If we made it this far with --force-run set to something it means an
	// alert by that name was never found, so we should error
	if config.ForceRun != "" {
		llog.Error("could not find alert with name given by --force-run")
		os.Exit(1)
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
