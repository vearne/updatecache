package updatecache

import (
	"sync"
	"sync/atomic"
)

type Item struct {
	key   any
	value any
	cond  *sync.Cond
	// data version
	version   uint32
	freshFlag bool
	cf        CalcTimeForNextUpdateFunc
}

func NewItem(key, value any) *Item {
	item := &Item{}
	item.key = key
	item.value = value
	atomic.StoreUint32(&item.version, 0)
	item.cond = sync.NewCond(new(sync.Mutex))
	item.freshFlag = true
	return item
}
