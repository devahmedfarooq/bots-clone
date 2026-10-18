package main

import (
	"bots/internal/dataset"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func datasetCommand(args []string) error {
	if len(args) < 3 || args[0] != "validate" {
		return errors.New("dataset validate DATA.jsonl EVALUATION.jsonl [NEW_EXPORT_DIRECTORY]")
	}
	records, report, e := dataset.Validate(args[1], args[2])
	if e != nil {
		write(report)
		return e
	}
	if len(args) > 4 {
		return errors.New("unexpected dataset arguments")
	}
	if len(args) == 4 {
		dir := args[3]
		if e = os.Mkdir(dir, 0700); e != nil {
			return e
		}
		ok := false
		defer func() {
			if !ok {
				os.RemoveAll(dir)
			}
		}()
		for _, split := range []string{"train", "validation", "test"} {
			f, e := os.Create(filepath.Join(dir, split+".jsonl"))
			if e != nil {
				return e
			}
			enc := json.NewEncoder(f)
			for _, r := range records {
				if r.Split == split {
					if e = enc.Encode(r); e != nil {
						f.Close()
						return e
					}
				}
			}
			if e = f.Close(); e != nil {
				return e
			}
		}
		hashes := map[string]string{}
		for _, split := range []string{"train", "validation", "test"} {
			raw, e := os.ReadFile(filepath.Join(dir, split+".jsonl"))
			if e != nil {
				return e
			}
			sum := sha256.Sum256(raw)
			hashes[split] = hex.EncodeToString(sum[:])
		}
		data, _ := json.MarshalIndent(map[string]any{"validation": report, "split_sha256": hashes}, "", "  ")
		if e = os.WriteFile(filepath.Join(dir, "data.lock.json"), data, 0600); e != nil {
			return e
		}
		ok = true
	}
	return write(report)
}
