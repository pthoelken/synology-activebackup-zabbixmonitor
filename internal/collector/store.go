package collector

import "sync"

type Store struct {
	mu       sync.RWMutex
	snapshot Snapshot
	hasValue bool
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Set(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snapshot
	s.hasValue = true
}

func (s *Store) Get() (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot, s.hasValue
}
