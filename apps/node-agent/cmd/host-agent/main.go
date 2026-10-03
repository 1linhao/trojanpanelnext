package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"trojan-panel-core/hostagent"
)

func main() {
	worker := len(os.Args) == 3 && os.Args[1] == "--run-update"
	if len(os.Args) != 1 && !worker {
		log.Fatal("usage: tp-host-agent [--run-update <job-id>]")
	}
	if worker && os.Geteuid() != 0 {
		log.Fatal("update worker requires root")
	}
	cleanup, err := hostagent.CleanupRequired()
	if err != nil {
		log.Fatal(err)
	}
	if cleanup {
		if worker {
			log.Fatal("maintenance cleanup is pending")
		}
		if err = hostagent.Cleanup(); err != nil {
			log.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(hostagent.Directory + "/config.json")
	if errors.Is(err, os.ErrNotExist) {
		if worker {
			log.Fatal("maintenance configuration is unavailable")
		}
		if err = hostagent.Cleanup(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	var config hostagent.Config
	if err = json.Unmarshal(data, &config); err != nil {
		log.Fatal(err)
	}
	if worker {
		server, err := hostagent.New(config, hostagent.Directory)
		if err != nil {
			log.Fatal(err)
		}
		signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stop()
		ctx, cancel := context.WithTimeout(signalCtx, 55*time.Minute)
		defer cancel()
		if err = server.RunUpdate(ctx, os.Args[2]); err != nil {
			log.Fatal("update worker state could not be completed")
		}
		return
	}
	if err = hostagent.Run(config); err != nil {
		log.Fatal(err)
	}
}
