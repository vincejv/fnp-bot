// This is an example program showing the usage of hellabot
package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-co-op/gocron/v2"
	hbot "github.com/whyrusleeping/hellabot"
	log "gopkg.in/inconshreveable/log15.v2"
)

var serv = flag.String("server", "irc.p2p-network.net:6668", "hostname and port for irc server to connect to")
var nick = flag.String("nick", "vincejvvv", "nickname for the bot")

func main() {
	flag.Parse()

	hijackSession := func(bot *hbot.Bot) {
		bot.HijackSession = true
	}
	channels := func(bot *hbot.Bot) {
		bot.Channels = []string{"#fnp-announce"}
	}
	irc, err := hbot.NewBot(*serv, *nick, hijackSession, channels)
	if err != nil {
		panic(err)
	}

	irc.AddTrigger(sayJoinMsg)
	//irc.AddTrigger(longTrigger)
	irc.Logger.SetHandler(log.StdoutHandler)
	// logHandler := log.LvlFilterHandler(log.LvlInfo, log.StdoutHandler)
	// or
	// irc.Logger.SetHandler(logHandler)
	// or
	// irc.Logger.SetHandler(log.StreamHandler(os.Stdout, log.JsonFormat()))

	// Start up bot (this blocks until we disconnect)
	// go irc.Run()

	// create a scheduler
	s, err := gocron.NewScheduler()
	if err != nil {
		// handle error
		fmt.Sprintln("Error in scheduler: %s", err.Error())
	}

	// add a job to the scheduler
	j, err := s.NewJob(
		gocron.DurationJob(
			10*time.Second,
		),
		gocron.NewTask(
			func(a string, b int) {
				// Request the HTML page.
				fmt.Println("Fetching FnP page")
				client := &http.Client{}
				req, err := http.NewRequest("GET", "https://fearnopeer.com/torrents?perPage=50", nil)
				req.Header.Set("Cookie", "remember_web_59ba36addc2b2f9401580f014c7f58ea4e30989d=eyJpdiI6InlNalFxdXJnL3pwM013cy9KSnZUZVE9PSIsInZhbHVlIjoiemtGUVA5TVlGbStrUjYwUmJHM0ttV3FnNUozaFlEQjFNMm1PWlRrMXdIL1pZT3A2QXZLVzljd2dOaGZNOEdyc0F1UGdXOVdOVVhPblpRaEhkTmJHamNMbE54dk9XVjAxaGhYOGM5enlXRm11aDYrMFFKem9IejJUR0dKQmVyaVJDQm1NcFAzT1pFbW1mektreE1oOEhOc0RscUwyTUxqcnRPZ0lLK2VnRXArL0ZXTFdrMWkrYnBGdElNeU0wYzlCYTNydFJYZDlON2NPMjAvbTQ0b29tU1R2SGo4dlhvdllISlNNWlB4WWI3cz0iLCJtYWMiOiI1ODMxN2VmZTQzZDdlOTBhNTMzMTA1MTJiODkxMjU0NTM2MmJiYjA2MDFmYjMxZTIyNjhhMzYzMzQzZjExMTBlIiwidGFnIjoiIn0%3D; XSRF-TOKEN=eyJpdiI6ImpOazBRa2poMnN4eTFXbEtvUFB1U2c9PSIsInZhbHVlIjoiUE5rNm1ORXNWU1E1K0YzajZLNW83b0VJNDhoekowWXFUaXp0T2dTWFQyRW9SMWQwMlg2SHViSHk4THpZTkMrRXY2YWVxOWI2dGl2NWVpaXpOT01QU3h3c3hQKzVMdmZjMUowTlBiV3l2UmpFR082c2FmTjZ0NXIvdnBHMmt4OXgiLCJtYWMiOiI1YmQxOGUyY2Y5Zjk5MzM3NWY1MzRkZTg4OTlmN2Y1N2U1ZDNiMWI5YzAwNmJmYTljNGU3OWYzYWMwMjY0ZDU4IiwidGFnIjoiIn0%3D; laravel_session=eyJpdiI6InBzNlI2TFZRVm9mcjJhb2xiam9vRFE9PSIsInZhbHVlIjoiRklXRjZGdWlob1VNWkZUKzAzSlBhQWNmb3dVdHZBZ3cxRjZTSDFRc0o5bjM4ZHlqdDVIU05hM25SbDM1NFhhcG1uaHA4VXNzNUZzSkQwWXVQbHgwc0tuajRVVXhSbnBZb2ZTanYyNm5TWVpFS3dhK2hEWVBlWTFJY3dJT0JkcU0iLCJtYWMiOiJmODYxZmU1NWI3ODAxZGY1Y2JlODI3NGRmMjJhMzBjZmE0OTIzNmE2NjJjMGFhOGZmNGM2MmE0NjZjOWI3ZDE5IiwidGFnIjoiIn0%3D; io=uhkjkJCKCekuyz48AABu")
				req.Header.Set("User-Agent", "Golang_IRC_Crawler_Bot/1.0")
				if err != nil {
					fmt.Println(err)
				}
				res, err := client.Do(req)
				if err != nil {
					fmt.Println(err)
				}
				defer res.Body.Close()
				if res.StatusCode != 200 {
					//log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
					fmt.Sprintln("Http error: %d", res.StatusCode)
				}

				// Load the HTML document
				doc, err := goquery.NewDocumentFromReader(res.Body)
				if err != nil {
					fmt.Sprintln("Error in scheduler: %s", err.Error())
				}

				// Find the review items
				doc.Find("body > main > article > div > section.panelV2.torrent-search__results > div > table > tbody > tr").Each(func(i int, s *goquery.Selection) {
					// For each item found, get the title
					title := s.Find(".torrent-search--list__name").Text()
					title = strings.TrimSpace(title)
					fmt.Printf("%s\n", title)
				})
			},
			"hello",
			1,
		),
	)
	if err != nil {
		fmt.Println("Error in job: " + err.Error())
	}
	// each job has a unique id
	fmt.Println(j.ID())

	// start the scheduler
	go s.Start()

	fmt.Println("Press CTRL+C to exit")
	select {} // block forever
}

// This trigger replies Hello when you say hello
var sayJoinMsg = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return strings.Contains(m.Raw, "#fnp-announce :End of /NAMES list.") && strings.Contains(m.From, ".p2p-network.net")
		//return m.Command == "PRIVMSG" && m.Content == "-info"
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		irc.Msg("#FnP-Announce", "Hello #FnP-Announce!! Will start announcing soon...")
		return false
	},
}
