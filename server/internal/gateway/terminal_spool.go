package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type terminalDelivery struct {
	EventID string          `json:"event_id"`
	Message json.RawMessage `json:"message"`
}

// terminalSpool stores one immutable terminal frame per Run. A server ack is
// the only normal deletion path, so a Gateway or WebSocket restart replays the
// exact same event identity and payload.
type terminalSpool struct {
	dir string
	mu  sync.Mutex
}

func newTerminalSpool(stateDir string) (*terminalSpool, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, fmt.Errorf("gateway state directory is required")
	}
	dir := filepath.Join(stateDir, "terminal-outbox")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create terminal spool: %w", err)
	}
	return &terminalSpool{dir: dir}, nil
}

func (s *terminalSpool) Put(eventID string, message any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("terminal event ID is required")
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal terminal message: %w", err)
	}
	delivery, err := json.Marshal(terminalDelivery{EventID: eventID, Message: raw})
	if err != nil {
		return fmt.Errorf("marshal terminal delivery: %w", err)
	}
	finalPath := s.path(eventID)
	if _, err := os.Stat(finalPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	temp, err := os.CreateTemp(s.dir, ".terminal-*.tmp")
	if err != nil {
		return fmt.Errorf("create terminal spool file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(delivery); err != nil {
		temp.Close()
		return fmt.Errorf("write terminal spool: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync terminal spool: %w", err)
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("commit terminal spool: %w", err)
	}
	return syncDirectory(s.dir)
}

func (s *terminalSpool) List() ([]terminalDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	result := make([]terminalDelivery, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var delivery terminalDelivery
		if err := json.Unmarshal(raw, &delivery); err != nil {
			return nil, fmt.Errorf("decode terminal spool %s: %w", entry.Name(), err)
		}
		if delivery.EventID == "" || len(delivery.Message) == 0 {
			return nil, fmt.Errorf("terminal spool %s is incomplete", entry.Name())
		}
		result = append(result, delivery)
	}
	return result, nil
}

func (s *terminalSpool) Ack(eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(eventID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(s.dir)
}

func (s *terminalSpool) path(eventID string) string {
	digest := sha256.Sum256([]byte(eventID))
	return filepath.Join(s.dir, hex.EncodeToString(digest[:])+".json")
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
