package followup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu    sync.Mutex
	path  string
	items map[string]ScheduledFollowUp
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, items: make(map[string]ScheduledFollowUp)}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.items); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Put(item ScheduledFollowUp) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.Token] = item
	return s.writeLocked()
}

func (s *Store) Take(token string) (ScheduledFollowUp, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[token]
	if !ok {
		return ScheduledFollowUp{}, false, nil
	}
	delete(s.items, token)
	if err := s.writeLocked(); err != nil {
		s.items[token] = item
		return ScheduledFollowUp{}, false, err
	}
	return item, true, nil
}

func (s *Store) writeLocked() error {
	raw, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".followups-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
