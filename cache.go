package updatecache

import (
	"sync"
	"sync/atomic"
	"time"
)

type GetValueFunc func() any

type LocalCache struct {
	m          map[any]*Item
	dataMu     sync.RWMutex
	waitUpdate bool
}

func NewCache(waitUpdate bool) *LocalCache {
	c := LocalCache{}
	c.m = make(map[any]*Item)
	c.waitUpdate = waitUpdate
	return &c
}

func (c *LocalCache) Set(key any, value any, d time.Duration) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	item, ok := c.m[key]
	if !ok {
		c.m[key] = NewItem(key, value)
	} else {
		item.cond.L.Lock()
		item.value = value
		item.cond.L.Unlock()
		atomic.AddUint32(&item.version, 1)
	}

	target := atomic.LoadUint32(&item.version)
	time.AfterFunc(d, func() {
		version := atomic.LoadUint32(&item.version)
		// item may be updated or refreshed
		if target < version {
			return
		}
		c.dataMu.Lock()
		defer c.dataMu.Unlock()
		delete(c.m, key)
	})
}

func (c *LocalCache) Get(key any) any {
	c.dataMu.RLock()
	item, ok := c.m[key]
	c.dataMu.RUnlock()

	if !ok {
		return nil
	}

	item.cond.L.Lock()
	defer item.cond.L.Unlock()
	if c.waitUpdate {
		for !item.freshFlag.IsTrue() {
			item.cond.Wait()
		}
	}
	return item.value
}

// use f to get value
func (c *LocalCache) UpdateAfter(key any, d time.Duration, f GetValueFunc) {
	c.dataMu.RLock()
	item, ok := c.m[key]
	c.dataMu.RUnlock()
	if !ok {
		return
	}

	target := atomic.LoadUint32(&item.version)
	time.AfterFunc(d, func() {
		version := atomic.LoadUint32(&item.version)
		// item may be updated or refreshed
		if target < version {
			return
		}
		// stale
		item.freshFlag.Set(false)

		item.cond.L.Lock()
		item.value = f()
		item.cond.L.Unlock()

		// fresh
		item.freshFlag.Set(true)
		item.cond.Broadcast()
	})
}
