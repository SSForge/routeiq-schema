package main

import (
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	c := NewCache(time.Minute)
	info := KeyInfo{OrgID: "org-1", WorkspaceID: "ws-1", Scope: "INGEST_ONLY"}
	c.Set("hash-abc", info)

	got, ok := c.Get("hash-abc")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.OrgID != "org-1" {
		t.Errorf("OrgID = %q, want org-1", got.OrgID)
	}
	if got.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID = %q, want ws-1", got.WorkspaceID)
	}
}

func TestCache_MissingKey(t *testing.T) {
	c := NewCache(time.Minute)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected cache miss for missing key")
	}
}

func TestCache_ExpiredEntry(t *testing.T) {
	c := NewCache(1 * time.Millisecond)
	c.Set("hash-exp", KeyInfo{OrgID: "org-x"})
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("hash-exp")
	if ok {
		t.Fatal("expected cache miss for expired entry")
	}
}
