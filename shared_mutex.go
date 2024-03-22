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

// Handshake checker
var wsHandshake = new(ItemIdCtr)

// Ping Pong watchdog timer
var pingPongWatchdog = new(ItemIdCtr)

// interrupts wait and reload for handshake
var interruptWnR = new(ItemIdCtr)

// interrupts PONG check and trickle
var interruptWSPong = new(ItemIdCtr)

// flag to check if fetching manually from API
var isFetchingManually = new(ItemIdCtr)

func initMutex() {
	lastItemId.Set(-1)
	lastFeatId.Set(-1)
	lastFLId.Set(-1)
	pingPongWatchdog.Set(0)
	refreshedPage.Set(0)
}

func (m *ItemIdCtr) Get() int64 {
	m.RLock()
	defer m.RUnlock()
	return m.me
}

func (m *ItemIdCtr) IsFlagged() bool {
	m.RLock()
	defer m.RUnlock()
	return m.me > 0
}

func (m *ItemIdCtr) Flag() {
	m.Lock()
	m.me = 1
	m.Unlock()
}

func (m *ItemIdCtr) Set(me int64) {
	m.Lock()
	m.me = me
	m.Unlock()
}

func (m *ItemIdCtr) Increment() {
	m.Lock()
	m.me = m.me + 1
	m.Unlock()
}

func (m *ItemIdCtr) Decrement() {
	m.Lock()
	m.me = m.me - 1
	m.Unlock()
}

func (m *ItemIdCtr) Reset() {
	m.Lock()
	m.me = 0
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
