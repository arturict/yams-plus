package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type State struct {
	SchemaVersion int               `json:"schemaVersion"`
	Phase         string            `json:"phase"`
	ConfigDigest  string            `json:"configDigest"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	Services      map[string]string `json:"services,omitempty"`
	Actions       []string          `json:"actions,omitempty"`
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func Load(path string) (State, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

func Save(path string, s State) error {
	s.SchemaVersion = 1
	s.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o640); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}
