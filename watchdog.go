package main

import (
	"context"
	"log"
	"time"
)

var wsWatchdogTimerInitVal = 35 // 35 seconds

func wsWatchdog(ctx context.Context, roomId string) {
	log.Println("Resetting WS Watchdog")
	wsWatchdogTimer := wsWatchdogTimerInitVal
	for wsWatchdogTimer > 0 {
		// poll and check every 1 sec if ws is already established
		time.Sleep(1 * time.Second)
		wsWatchdogTimer = wsWatchdogTimer - 1
		if pingPongWatchdog.IsFlagged() || interruptWSPong.IsFlagged() { // also reset of interrupted
			interruptWSPong.Reset()
			pingPongWatchdog.Reset()
			wsWatchdogTimer = wsWatchdogTimerInitVal // trickle to initVal
		}
	}
	go reloadChatPage(ctx, roomId, "WS PingPong failed, reloading page")
	wsWatchdog(ctx, roomId) //lint:ignore SA5007 Expected to recurse infinitely as this is a watchdog timer, thread sleeps are implemented
}
