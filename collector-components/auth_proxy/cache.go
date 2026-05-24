package main

import (
	"sync"
	"time"
)

// KeyInfo holds the resolved identity for a validated API key.
type KeyInfo struct {
	OrgID       string
	WorkspaceID string // empty if the key has no workspace scope
	Scope       string
}

type cacheEntry struct {
	info KeyInfo
	exp  time.Time
}

// Cache is an in-process TTL cache keyed by SHA-256 hex of the raw API key.
type Cache struct {
	m   sync.Map
	ttl time.Duration
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl}
}

// Get returns the cached KeyInfo for hash. Returns false if missing or expired.
func (c *Cache) Get(hash string) (KeyInfo, bool) {
	v, ok := c.m.Load(hash)
	if !ok {
		return KeyInfo{}, false
	}
	e := v.(cacheEntry)
	if time.Now().After(e.exp) {
		c.m.Delete(hash)
		return KeyInfo{}, false
	}
	return e.info, true
}

// Set stores info under hash with the configured TTL.
func (c *Cache) Set(hash string, info KeyInfo) {
	c.m.Store(hash, cacheEntry{info: info, exp: time.Now().Add(c.ttl)})
}
