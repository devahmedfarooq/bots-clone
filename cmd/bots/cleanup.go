package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"bots/internal/lock"
	"bots/internal/service"
	"bots/internal/state"
)

func cleanup(c appConfig, args []string) error {
	f := flags("cleanup")
	before := f.String("before", "", "retain records on/after RFC3339 timestamp")
	keepIDs := f.String("keep", "", "comma-separated run IDs to preserve")
	apply := f.Bool("apply", false, "apply the printed deletion plan")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected cleanup arguments")
	}
	cutoff, err := time.Parse(time.RFC3339, *before)
	if err != nil {
		return errors.New("cleanup requires -before RFC3339")
	}
	unlock, err := lock.Acquire(filepath.Join(c.Home, "inference.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	s, err := state.Open(filepath.Join(c.Home, "metadata.db"))
	if err != nil {
		return err
	}
	defer s.Close()
	keep := map[string]bool{}
	for _, id := range strings.Split(*keepIDs, ",") {
		if id != "" {
			keep[id] = true
		}
	}
	jobs, err := s.Checkpoints("job")
	if err != nil {
		return err
	}
	jobRuns := map[string]string{}
	for _, raw := range jobs {
		var j service.Job
		if err = json.Unmarshal(raw, &j); err != nil {
			return err
		}
		if j.Run != nil {
			jobRuns[j.ID] = j.Run.ID
		}
	}
	flows, err := s.Checkpoints("workflow")
	if err != nil {
		return err
	}
	for _, raw := range flows {
		var flow service.Flow
		if err = json.Unmarshal(raw, &flow); err != nil {
			return err
		}
		for _, step := range flow.Steps {
			if run := jobRuns[step.Job]; run != "" {
				keep[run] = true
			}
		}
	}
	ids, err := s.RetentionPlan(cutoff, keep)
	if err != nil {
		return err
	}
	plan := map[string]any{"before": cutoff, "delete_runs": ids, "preserve": "running runs, requested IDs, workflow references, all model/adapter/package artifacts", "applied": false}
	if *apply {
		if err = s.DeleteRuns(ids); err != nil {
			return err
		}
		plan["applied"] = true
	}
	return write(plan)
}
