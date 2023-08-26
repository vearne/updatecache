package updatecache

import (
	"sync"
	"time"
)

type Item struct {
	key   any
	value any
	cond  *sync.Cond
	// update in progress
	updatingFlag      bool
	cf                CalcTimeForNextUpdateFunc
	expireTimer       *time.Timer
	updateTimer       *time.Timer
	configUpdaterOnce sync.Once
	dataVersion       uint64
}

func NewItem(key, value any) *Item {
	item := &Item{}
	item.key = key
	item.value = value
	item.cond = sync.NewCond(new(sync.Mutex))
	item.updatingFlag = false
	item.dataVersion = 0
	return item
}
