package main

import (
	"context"
	"log"
	"time"
)

var wsWatchdogTimerInitVal = 35 // 35 seconds
var wsWatchdogTimer = wsWatchdogTimerInitVal

func wsWatchdog(ctx context.Context, roomId string) {
	log.Println("Starting WS Watchdog")
	for wsWatchdogTimer > 0 {
		// poll and check every 1 sec if ws is already established
		time.Sleep(1 * time.Second)
		wsWatchdogTimer = wsWatchdogTimer - 1
		if pingPongWatchdog.Get() >= 1 {
			// ws Got Ponged
			pingPongWatchdog.Reset()
			wsWatchdogTimer = wsWatchdogTimerInitVal
			return
		}
	}
	go reloadChatPage(ctx, roomId, "WS PingPong failed, reloading page")
	wsWatchdog(ctx, roomId)
}
