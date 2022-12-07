package updatecache

import (
	slog "github.com/vearne/simplelog"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, 3*time.Second)
	cache.UpdateAfter(key, time.Second, func() any {
		slog.Debug("get value")
		return 2
	})
	time.Sleep(2 * time.Second)
	value := cache.Get(key)
	if 2 != value {
		t.Errorf("expected %d, got: %v", 2, value)
	}
}

func TestSet2(t *testing.T) {
	key := "aaa"
	cache := NewCache(true)
	cache.Set(key, 1, 3*time.Second)
	go func() {
		time.Sleep(time.Second)
		cache.Set(key, 2, 5*time.Second)
	}()
	time.Sleep(2 * time.Second)
	value := cache.Get(key)
	if 2 != value {
		t.Errorf("expected %d, got: %v", 2, value)
	}
}
