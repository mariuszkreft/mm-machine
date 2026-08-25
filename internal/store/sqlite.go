package store

import "errors"

// Open returns a SQLite-backed Store. The SQLite implementation is owned by the
// store worker; until it lands, callers fall back to the in-memory store.
func Open(path string) (Store, error) {
	_ = path
	return nil, errors.New("store: sqlite backend not implemented yet")
}
