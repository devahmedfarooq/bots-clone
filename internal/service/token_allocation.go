package service

import (
	"errors"
	"fmt"
)

func (a *Autonomy) allocateTokens(key string, required int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := cloneBoard(a.board)
	for i := range b.Cards {
		c := &b.Cards[i]
		if c.Key != key || c.Status != InProgress {
			continue
		}
		if required <= c.Reserved {
			return c.Reserved, nil
		}
		g := &b.Goals[goalIndex(b, c.Goal)]
		extra := min(max(required-c.Reserved, c.Reserved), g.TokenLimit-g.Tokens)
		if extra <= 0 {
			return c.Reserved, nil
		}
		c.Reserved += extra
		g.Tokens += extra
		record(c, "manager", fmt.Sprintf("Increased run token reservation to %d; unused tokens return to the goal on completion.", c.Reserved))
		if err := a.save(b); err != nil {
			return 0, err
		}
		return c.Reserved, nil
	}
	return 0, errors.New("task is no longer active; cannot allocate more tokens")
}
