package facts

import "time"

// StoreMeta holds metadata about a fact store (incident context).
type StoreMeta struct {
	Model    string    `yaml:"model"`
	Version  string    `yaml:"version"`
	Incident string    `yaml:"incident"`
	Started  time.Time `yaml:"started"`
}

// Store is an append-only fact store. It may be in-memory or disk-backed.
// For disk-backed stores, call Save() (or AppendAndSave()) to persist.
type Store struct {
	Meta  StoreMeta
	facts map[string][]Fact
	path  string // empty for in-memory stores
}

// Fact is a single observed value for a named key on a component.
type Fact struct {
	Key       string
	Value     any
	Collector string
	At        time.Time
	Note      string
	Raw       string
}
