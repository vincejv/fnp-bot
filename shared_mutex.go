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

// Track last item fetched
var lastItemId = new(ItemIdCtr)

// Track last feature item fetched
var lastFeatId = new(ItemIdCtr)

// Track last freeleech item fetched
var lastFLId = new(ItemIdCtr)

// Store cookies from login
var cookieJar = new(CookieJar)

// Check if page has refreshed
var refreshedPage = new(ItemIdCtr)

func initMutex() {
	lastItemId.Set(-1)
	lastFeatId.Set(-1)
	lastFLId.Set(-1)
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
