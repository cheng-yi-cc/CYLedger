package marketquotes

import (
	"context"
	"errors"
	"time"
)

type candidateTask func(context.Context) ([]Candidate, error)
type candidateResult struct {
	items []Candidate
	err   error
	index int
}

func searchLane(market string) int {
	if market == "CRYPTO" {
		return 1
	}
	if market == "" {
		return 2
	}
	return 0
}

// Each caller owns its cancellation. A cancelled UI request cannot cancel another
// caller's identical lookup. The shared operation still has one hard deadline.
func (s *Service) coalescedSearch(ctx context.Context, key string, lane int, task candidateTask) ([]Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := s.searchFlights.DoChan(key, func() (interface{}, error) {
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.SearchTimeout)
		defer cancel()
		select {
		case s.searchSlots[lane] <- struct{}{}:
		case <-work.Done():
			return nil, work.Err()
		}
		defer func() { <-s.searchSlots[lane] }()
		return task(work)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return nil, result.Err
		}
		return append([]Candidate{}, result.Val.([]Candidate)...), nil
	}
}

// Results are deterministic, even though sources run in parallel. A successful
// source gets 300 ms for its alternatives; incomplete searches are never cached.
func parallelCandidates(ctx context.Context, tasks []candidateTask) ([]Candidate, error) {
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	responses := make(chan candidateResult, len(tasks))
	for i, task := range tasks {
		go func(i int, task candidateTask) { items, err := task(work); responses <- candidateResult{items, err, i} }(i, task)
	}
	parts := make([][]Candidate, len(tasks))
	var failures []error
	var grace <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	remaining := len(tasks)
	for remaining > 0 {
		select {
		case <-ctx.Done():
			failures = append(failures, ctx.Err())
			remaining = 0
		case <-grace:
			failures = append(failures, errors.New("partial catalogue"))
			remaining = 0
		case response := <-responses:
			remaining--
			parts[response.index] = response.items
			if response.err != nil {
				failures = append(failures, response.err)
			}
			if len(response.items) > 0 && timer == nil {
				timer = time.NewTimer(300 * time.Millisecond)
				grace = timer.C
			}
		}
	}
	items := make([]Candidate, 0)
	for _, part := range parts {
		items = append(items, part...)
	}
	return items, errors.Join(failures...)
}

func evictSearchEntry(cache map[string]searchEntry) {
	var oldest string
	var at time.Time
	for key, entry := range cache {
		if at.IsZero() || entry.at.Before(at) {
			oldest, at = key, entry.at
		}
	}
	delete(cache, oldest)
}
