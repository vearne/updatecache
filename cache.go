package updatecache

import (
	"context"
	slog "github.com/vearne/simplelog"
	"golang.org/x/time/rate"
	"sync"
	"time"
)

type GetValueFunc func() (value any, err error)

//Calculate the waiting time until the next data update
type CalcTimeForNextUpdateFunc func(value any) time.Duration

type LocalCache struct {
	m          map[any]*Item
	dataMu     sync.RWMutex
	waitUpdate bool
	// limiter Limit the execution frequency of GetValueFunc
	limiter *rate.Limiter
}

func NewCache(waitUpdate bool, opts ...Option) *LocalCache {
	c := LocalCache{}
	c.m = make(map[any]*Item)
	c.waitUpdate = waitUpdate
	// Loop through each option
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

func (c *LocalCache) FirstLoad(key any, defaultValue any, gf GetValueFunc, d time.Duration) any {
	item := NewItem(key, defaultValue)
	item.cond.L.Lock()
	item.updatingFlag = true
	item.cond.L.Unlock()

	c.dataMu.Lock()
	_, ok := c.m[key]
	if !ok {
		c.m[key] = item
		if d >= 0 {
			timer := time.AfterFunc(d, func() {
				c.dataMu.Lock()
				defer c.dataMu.Unlock()
				delete(c.m, key)
			})
			item.expireTimer = timer
		}
		go func() {
			if c.limiter != nil {
				c.limiter.Wait(context.Background())
			}
			// get value from backend
			value, err := gf()
			if err == nil {
				item.cond.L.Lock()
				item.value = value
				item.updatingFlag = false
				item.cond.L.Unlock()
				item.cond.Broadcast()
			}
		}()
	}
	c.dataMu.Unlock()

	return c.Get(key)
}

/*
	If d is less than 0, the item does not expire
*/
func (c *LocalCache) SetIfNotExist(key any, value any, d time.Duration) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	_, ok := c.m[key]
	if !ok {
		item := NewItem(key, value)
		c.m[key] = item
		if d >= 0 {
			timer := time.AfterFunc(d, func() {
				c.dataMu.Lock()
				defer c.dataMu.Unlock()
				delete(c.m, key)
			})

			item.expireTimer = timer
		}
	}
}

/*
	If d is less than 0, the item does not expire
*/
func (c *LocalCache) Set(key any, value any, d time.Duration) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()

	item, ok := c.m[key]
	if !ok {
		item = NewItem(key, value)
		c.m[key] = item
	} else {
		item.cond.L.Lock()
		item.value = value
		item.updatingFlag = false
		item.cond.L.Unlock()
		item.cond.Broadcast()
	}

	if d >= 0 {
		timer := time.AfterFunc(d, func() {
			c.dataMu.Lock()
			defer c.dataMu.Unlock()
			delete(c.m, key)
		})
		if item.expireTimer != nil {
			if !item.expireTimer.Stop() {
				<-item.expireTimer.C
			}
			item.expireTimer.Reset(d)
		} else {
			item.expireTimer = timer
		}
	}
}

func (c *LocalCache) Contains(key any) (ok bool) {
	_, ok = c.getItem(key)
	return ok
}

func (c *LocalCache) getItem(key any) (item *Item, ok bool) {
	c.dataMu.RLock()
	item, ok = c.m[key]
	c.dataMu.RUnlock()
	return item, ok
}

func (c *LocalCache) Size() int {
	c.dataMu.RLock()
	defer c.dataMu.RUnlock()
	return len(c.m)
}

func (c *LocalCache) Remove(key any) {
	c.dataMu.Lock()
	defer c.dataMu.Unlock()
	delete(c.m, key)
}

func (c *LocalCache) StopLaterUpdate(key any) {
	item, ok := c.getItem(key)
	if !ok {
		return
	}

	item.cond.L.Lock()
	item.cf = nil
	item.cond.L.Unlock()
	item.updateTimer.Stop()
}

func (c *LocalCache) Get(key any) any {
	item, ok := c.getItem(key)
	if !ok {
		return nil
	}

	item.cond.L.Lock()
	defer item.cond.L.Unlock()
	if c.waitUpdate {
		for item.updatingFlag {
			item.cond.Wait()
		}
	}
	return item.value
}

/*
	Notice: Item and CalcTimeForNextUpdateFunc, GetValueFunc will only be bound once.
	1. Calculate the waiting time until the next data update with CalcTimeForNextUpdateFunc
	2. then use GetValueFunc to get value
	3. update item
*/
func (c *LocalCache) DynamicUpdateLater(key any, cf CalcTimeForNextUpdateFunc, gf GetValueFunc) {
	item, ok := c.getItem(key)
	if !ok {
		return
	}
	item.cond.L.Lock()
	if item.cf == nil {
		item.cf = cf
		c.UpdateLater(key, cf(item.value), gf)
	}
	item.cond.L.Unlock()
}

//  1. wait d
//  2. then use GetValueFunc to get value
//  3. update item
func (c *LocalCache) UpdateLater(key any, d time.Duration, getValueFunc GetValueFunc) {
	cacheItem, ok := c.getItem(key)
	if !ok {
		return
	}

	timer := time.AfterFunc(d, func() {
		slog.Debug("[start]update item after %v", d)
		item, ok := c.getItem(key)
		// item may be removed
		if !ok {
			return
		}

		item.cond.L.Lock()
		item.updatingFlag = true
		item.cond.L.Unlock()

		if c.limiter != nil {
			c.limiter.Wait(context.Background())
		}
		value, err := getValueFunc()

		if err == nil {
			item.cond.L.Lock()
			item.value = value
			item.updatingFlag = false
			slog.Debug("item:%p, f():%v", item, item.value)
			item.cond.L.Unlock()

			item.cond.Broadcast()
		}

		// plan for next update
		if item.cf != nil {
			c.UpdateLater(key, item.cf(value), getValueFunc)
		}

		slog.Debug("[end]update item after %v", d)
	})

	if cacheItem.updateTimer != nil {
		if !cacheItem.updateTimer.Stop() {
			<-cacheItem.updateTimer.C
		}
		cacheItem.updateTimer.Reset(d)
	} else {
		cacheItem.updateTimer = timer
	}
}
