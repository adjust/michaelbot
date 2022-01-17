package deploy

import (
	"sync"
	"time"
)

type InMemoryStore struct {
	qmu sync.RWMutex
	hmu sync.RWMutex
	m   map[string]Queue
	h   map[string][]Deploy
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		m: make(map[string]Queue),
		h: make(map[string][]Deploy),
	}
}

func (s *InMemoryStore) GetQueue(key string) (q Queue) {
	s.qmu.RLock()

	q, ok := s.m[key]
	if !ok {
		q = NewEmptyQueue()
		s.m[key] = q
	}

	s.qmu.RUnlock()

	return q
}

func (s *InMemoryStore) SetQueue(key string, q Queue) {
	s.qmu.Lock()

	s.m[key] = q

	s.qmu.Unlock()
}

func (s *InMemoryStore) All(key string) []Deploy {
	s.hmu.RLock()

	deploys := make([]Deploy, len(s.h[key]))

	copy(deploys, s.h[key])

	s.hmu.RUnlock()

	return deploys
}

func (s *InMemoryStore) Since(key string, startTime time.Time) []Deploy {
	s.hmu.RLock()

	history, ok := s.h[key]

	if !ok {
		return nil
	}

	s.hmu.RUnlock()

	i := len(history)

	for ; i > 0; i-- {
		if history[i-1].StartedAt.Before(startTime) {
			break
		}
	}

	if i == len(history) {
		return nil
	}

	return history[i:]
}

func (s *InMemoryStore) AddToHistory(key string, d Deploy) {
	s.hmu.Lock()

	h, ok := s.h[key]
	if !ok {
		h = make([]Deploy, 0)
	}

	h = append(h, d)

	s.h[key] = h

	s.hmu.Unlock()
}
