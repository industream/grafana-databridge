package cache

import (
	"testing"
	"time"
)

func TestStore_SetAndGet(t *testing.T) {
	store := NewStore[string](5 * time.Minute)

	store.Set("key1", "value1")

	val, ok := store.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}
}

func TestStore_GetMissing(t *testing.T) {
	store := NewStore[string](5 * time.Minute)

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected missing key to return false")
	}
}

func TestStore_Expiration(t *testing.T) {
	store := NewStore[string](1 * time.Millisecond)

	store.Set("key1", "value1")
	time.Sleep(5 * time.Millisecond)

	_, ok := store.Get("key1")
	if ok {
		t.Error("expected expired key to return false")
	}
}

func TestStore_Invalidate(t *testing.T) {
	store := NewStore[string](5 * time.Minute)

	store.Set("key1", "value1")
	store.Invalidate("key1")

	_, ok := store.Get("key1")
	if ok {
		t.Error("expected invalidated key to return false")
	}
}

func TestStore_Clear(t *testing.T) {
	store := NewStore[string](5 * time.Minute)

	store.Set("key1", "value1")
	store.Set("key2", "value2")
	store.Clear()

	_, ok1 := store.Get("key1")
	_, ok2 := store.Get("key2")
	if ok1 || ok2 {
		t.Error("expected all keys to be cleared")
	}
}

func TestStore_Overwrite(t *testing.T) {
	store := NewStore[int](5 * time.Minute)

	store.Set("counter", 1)
	store.Set("counter", 2)

	val, ok := store.Get("counter")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}
}
