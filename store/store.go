package store

import (
	"Dynamo/config"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type JSONStore[T any] struct {
	file  string
	mu    sync.RWMutex
	cache *T
}

func NewJSONStore[T any](file string) *JSONStore[T] {
	filePath := filepath.Join(config.GetConfig().System.DataDirectory, file)
	os.MkdirAll(config.GetConfig().System.DataDirectory, 0o755)
	return &JSONStore[T]{file: filePath}
}

func (s *JSONStore[T]) Load() (T, error) {
	s.mu.RLock()
	if s.cache != nil {
		defer s.mu.RUnlock()
		return *s.cache, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil {
		return *s.cache, nil
	}

	var data T

	bytes, err := os.ReadFile(s.file)
	if err != nil {
		if !os.IsNotExist(err) {
			return data, err
		}
	}

	if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &data); err != nil {
			return data, err
		}
	}

	s.cache = &data
	return data, nil
}

func (s *JSONStore[T]) Save(data T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.file + ".tmp"

	if err := os.WriteFile(tmp, bytes, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmp, s.file); err != nil {
		return err
	}

	s.cache = &data
	return nil
}

func (s *JSONStore[T]) Update(fn func(*T) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var data T

	bytes, err := os.ReadFile(s.file)
	if err == nil && len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &data); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := fn(&data); err != nil {
		return err
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmp, s.file); err != nil {
		return err
	}

	s.cache = &data
	return nil
}

func (s *JSONStore[T]) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var data T

	bytes, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			s.cache = &data
			return nil
		}
		return err
	}

	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	s.cache = &data
	return nil
}
