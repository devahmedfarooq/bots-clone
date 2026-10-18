package main

import (
	"encoding/json"
	"os"

	"bots/plugin"
)

func main() {
	err := plugin.Serve(os.Stdin, os.Stdout, func(raw json.RawMessage) (string, error) {
		var args struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		return args.Text, nil
	})
	if err != nil {
		os.Exit(1)
	}
}
