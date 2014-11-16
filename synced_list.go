package main

import "sync"

type syncedlist struct {
	list []string
	sync.Mutex
}

func (sl *syncedlist) add(item string) {
	sl.Lock()
	sl.list = append(sl.list, item)
	sl.Unlock()
}
