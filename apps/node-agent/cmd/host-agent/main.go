package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"trojan-panel-core/hostagent"
)

func main() {
	cleanup, err := hostagent.CleanupRequired()
	if err != nil {
		log.Fatal(err)
	}
	if cleanup {
		if err = hostagent.Cleanup(); err != nil {
			log.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(hostagent.Directory + "/config.json")
	if errors.Is(err, os.ErrNotExist) {
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
	if err = hostagent.Run(config); err != nil {
		log.Fatal(err)
	}
}
