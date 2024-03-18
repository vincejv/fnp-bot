package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-co-op/gocron/v2"
	_ "github.com/mattn/go-sqlite3"
	hbot "github.com/whyrusleeping/hellabot"
	logi "gopkg.in/inconshreveable/log15.v2"
)

const LAST_ANNOUNCE_SETTING_ID int = 1

var serv = os.Getenv("IRC_SERVER")
var nick = os.Getenv("BOT_NICK")
var ircChannel = os.Getenv("IRC_CHANNEL")
var ircPassword = os.Getenv("IRC_BOT_PASSWORD")

// var ircDomainTrigger = os.Getenv("IRC_DOMAIN_TRIGGER") // Trigger for hello message and nickserv auth
var crawlerCookie = os.Getenv("CRAWLER_COOKIE")
var fetchSec = os.Getenv("FETCH_SEC")
var initLastTorrentId = os.Getenv("INIT_TORRENT_ID")

type Announce struct {
	TorrentId   int
	Name        string
	Size        string
	CreatedDate time.Time
	Category    string
	Type        string
	Uploader    string
	URL         string
	RawLine     string
}

type Setting struct {
	id    int
	name  string
	value int
}

func main() {
	flag.Parse()

	// Prepare SQLite Database
	db := openDb()

	// Prepare IRC Bot
	irc := createIRCBot()
	//irc.AddTrigger(nickServAuth)

	// create a scheduler
	scheduler := createScheduler()

	// add a job to the scheduler
	// each job has a unique id
	fetchSecNum, _ := strconv.Atoi(fetchSec)
	scheduleFetchJob(scheduler, fetchSecNum, db, irc)

	// start the scheduler routine
	go scheduler.Start()
	log.Println("Press CTRL+C to exit")

	// Start up bot (this blocks until we disconnect)
	irc.Run()
	//select {} // block forever
}

func createIRCBot() *hbot.Bot {
	botConfig := func(bot *hbot.Bot) {
		bot.SSL = true
		bot.Password = ircPassword // SASL (if enabled) or ZNC password
	}
	channels := func(bot *hbot.Bot) {
		bot.Channels = []string{ircChannel}
	}

	irc, err := hbot.NewBot(serv, nick, botConfig, channels)
	if err != nil {
		log.Fatal(err)
	}

	// IRC trace and logging
	irc.Logger.SetHandler(logi.StdoutHandler)
	return irc
}

func createScheduler() gocron.Scheduler {
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		// handle error
		log.Printf("Error while creating scheduler: %s\n", err.Error())
	}
	return scheduler
}

func scheduleFetchJob(scheduler gocron.Scheduler, fetchSecNum int, db *sql.DB, irc *hbot.Bot) {
	j, err := scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(fetchSecNum)*time.Second,
		),
		gocron.NewTask(
			func(a string, b int) {
				// Request the HTML page.
				client := &http.Client{}
				req, err := http.NewRequest("GET", "https://fearnopeer.com/torrents?perPage=50", nil)
				req.Header.Set("Cookie", crawlerCookie)
				req.Header.Set("User-Agent", "Golang_IRC_Crawler_Bot/1.0")
				if err != nil {
					log.Println(err)
				}
				res, err := client.Do(req)
				if err != nil {
					log.Println(err)
				}
				defer res.Body.Close()
				if res.StatusCode != 200 {
					log.Printf("Http error: %d\n", res.StatusCode)
				}

				// Load the HTML document
				doc, err := goquery.NewDocumentFromReader(res.Body)
				if err != nil {
					log.Printf("Error in scheduler: %s", err.Error())
				}

				var fetchedTors []Announce

				// Scrape the items
				doc.Find("body > main > article > div > section.panelV2.torrent-search__results > div > table > tbody > tr").Each(func(i int, s *goquery.Selection) {

					torrentIdStr, _ := s.Attr("data-torrent-id")
					torrentId, _ := strconv.Atoi(torrentIdStr)
					categoryIdStr, _ := s.Attr("data-category-id")
					categoryId, _ := strconv.Atoi(categoryIdStr)
					typeIdStr, _ := s.Attr("data-type-id")
					typeId, _ := strconv.Atoi(typeIdStr)
					url := fmt.Sprintf("https://fearnopeer.com/torrents/%d", torrentId)
					title := strings.TrimSpace(s.Find("a.torrent-search--list__name").Text())
					uploader := strings.TrimSpace(s.Find("span.torrent-search--list__uploader").Text())

					// remove parenthesis for anonymous uploader
					if strings.Contains(uploader, "(Anonymous)") {
						uploader = "Anonymous"
					}

					size := strings.TrimSpace(s.Find("td.torrent-search--list__size").Text())
					announceLine := fmt.Sprintf("Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]", getCategoryFriendlyStr(categoryId), getTypeFriendlyStr(typeId), title, size, uploader, url)

					// Store fetched torrents temporarily for processing
					announceDoc := Announce{TorrentId: torrentId, Name: title, Size: size, Category: getCategoryFriendlyStr(categoryId),
						Type: getTypeFriendlyStr(typeId), Uploader: uploader, URL: url, RawLine: announceLine, CreatedDate: time.Now()}
					fetchedTors = append(fetchedTors, announceDoc)
				})

				// refresh and fetch announce setting from DB
				announceSetting := getSetting(db, LAST_ANNOUNCE_SETTING_ID)
				lastTorrentId := announceSetting.value

				// Must sort in ascending so next block will work correctly
				sort.Slice(fetchedTors, func(tori, torj int) bool {
					return fetchedTors[tori].TorrentId < fetchedTors[torj].TorrentId
				})

				// Examine fetched torrents and push to IRC based on last send item id
				for _, tor := range fetchedTors {
					if tor.TorrentId > lastTorrentId {
						log.Println(tor.RawLine)
						irc.Msg(ircChannel, tor.RawLine)
						lastTorrentId = tor.TorrentId
					}
				}

				// save last announce setting to DB
				updateSetting(db, LAST_ANNOUNCE_SETTING_ID, lastTorrentId)
			},
			"fetchJob",
			1,
		),
	)
	if err != nil {
		log.Println("Error creating job: " + err.Error())
	}

	log.Println("Cron Job created with id:" + j.ID().String())
}

func openDb() *sql.DB {
	db, err := sql.Open("sqlite3", "/config/announce.db")

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	sts := `CREATE TABLE IF NOT EXISTS announce(id INTEGER PRIMARY KEY, name TEXT, value INT);`
	_, err = db.Exec(sts)

	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO announce(id, name, value) VALUES(?, 'lastTorrentId', ?)`, LAST_ANNOUNCE_SETTING_ID, initLastTorrentId)
	if err != nil {
		log.Println(err)
		log.Println("Existing `lastTorrentId` is set, ignoring `INIT_TORRENT_ID` setting, it's only used for initialization of DB")
	}
	return db
}

func getSetting(db *sql.DB, settingId int) *Setting {
	var searches []Setting
	row, err := db.Query("SELECT * FROM announce WHERE id = ? LIMIT ?", settingId, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()
	for row.Next() { // Iterate and fetch the records from result cursor
		item := Setting{}
		err := row.Scan(&item.id, &item.name, &item.value)
		if err != nil {
			log.Fatal(err)
		}
		searches = append(searches, item)
	}
	if len(searches) == 0 {
		log.Printf("Setting with index %d was not found", settingId)
		return nil
	}
	return &searches[0]
}

func updateSetting(db *sql.DB, id int, value int) {
	db.Exec("UPDATE announce SET value = ? WHERE id = ?", value, id)
}

func getCategoryFriendlyStr(catId int) string {
	switch catId {
	case 1:
		return "Movies"
	case 2:
		return "TV"
	case 3:
		return "Music"
	case 4:
		return "Anime"
	case 5:
		return "Games"
	case 6:
		return "Apps"
	case 7:
		return "Sport"
	case 8:
		return "Assorted"
	default:
		return "Unknown"
	}
}

func getTypeFriendlyStr(typeId int) string {
	switch typeId {
	case 1:
		return "Full Disc"
	case 2:
		return "Remux"
	case 3:
		return "Encode"
	case 4:
		return "WEB-DL"
	case 5:
		return "WEBRip"
	case 6:
		return "HDTV"
	case 7:
		return "FLAC"
	case 11:
		return "MP3"
	case 12:
		return "Mac"
	case 13:
		return "Windows"
	case 17:
		return "PlayStation"
	case 14:
		return "AudioBooks"
	case 15:
		return "Books"
	default:
		return "Misc"
	}
}

// This trigger replies Hello when you say hello
// var sayJoinMsg = hbot.Trigger{
// 	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
// 		return strings.Contains(m.Raw, ircChannel+" :End of /NAMES list.") && strings.Contains(m.From, ircDomainTrigger)
// 		//return m.Command == "PRIVMSG" && m.Content == "-info"
// 	},
// 	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
// 		irc.Msg(ircChannel, fmt.Sprintf("Hello %s!! Will start announcing soon...", ircChannel))
// 		return false
// 	},
// }

// This trigger replies Hello when you say hello
// var nickServAuth = hbot.Trigger{
// 	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
// 		return strings.Contains(m.Raw, " :End of message of the day.") && strings.Contains(m.From, ircDomainTrigger)
// 		//return m.Command == "PRIVMSG" && m.Content == "-info"
// 	},
// 	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
// 		irc.Msg("NickServ", "identify "+zncPassword)
// 		return false
// 	},
// }
