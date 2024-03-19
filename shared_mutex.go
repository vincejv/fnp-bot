package main

import (
	"sync"
)

type ItemIdCtr struct {
	sync.RWMutex
	me int64
}

type CookieJar struct {
	sync.RWMutex
	cookieVal string
}

var lastItemId = new(ItemIdCtr)
var cookieJar = new(CookieJar)
var refreshedPage = new(ItemIdCtr)

func initMutex() {
	lastItemId.Set(-1)
	refreshedPage.Set(0)
}

func (m *ItemIdCtr) Get() int64 {
	m.RLock()
	defer m.RUnlock()
	return m.me
}

func (m *ItemIdCtr) Set(me int64) {
	m.Lock()
	m.me = me
	m.Unlock()
}

func (m *CookieJar) Get() string {
	m.RLock()
	defer m.RUnlock()
	return m.cookieVal
}

func (m *CookieJar) Set(cookieVal string) {
	m.Lock()
	m.cookieVal = cookieVal
	m.Unlock()
}
