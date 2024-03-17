package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-co-op/gocron/v2"
	hbot "github.com/whyrusleeping/hellabot"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	logi "gopkg.in/inconshreveable/log15.v2"
)

var serv = os.Getenv("IRC_SERVER")
var nick = os.Getenv("BOT_NICK")
var ircChannel = os.Getenv("IRC_CHANNEL")
var ircPassword = os.Getenv("IRC_BOT_PASSWORD")
var ircDomainTrigger = os.Getenv("IRC_DOMAIN_TRIGGER") // Trigger for hello message and nickserv auth
var mongoConnectionStr = os.Getenv("MONGODB_CONN_STRING")
var dbName = os.Getenv("DB_NAME")
var crawlerCookie = os.Getenv("CRAWLER_COOKIE")
var fetchSec = os.Getenv("FETCH_SEC")

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

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoConnectionStr))
	if err != nil {
		log.Println("Error mongo connection")
	}
	mongoColl := mongoClient.Database(dbName).Collection("announces")
	//var result bson.M
	//mongoColl.FindOne(context.TODO(), bson.D{{"title", "test"}}).Decode(&result)

	hijackSession := func(bot *hbot.Bot) {
		bot.SSL = true
		bot.SASL = true
		bot.Password = ircPassword
	}
	channels := func(bot *hbot.Bot) {
		bot.Channels = []string{ircChannel}
	}
	irc, err := hbot.NewBot(serv, nick, hijackSession, channels)
	if err != nil {
		panic(err)
	}

	irc.AddTrigger(nickServAuth)
	irc.AddTrigger(sayJoinMsg)

	irc.Logger.SetHandler(logi.StdoutHandler)

	// create a scheduler
	s, err := gocron.NewScheduler()
	if err != nil {
		// handle error
		log.Printf("Error in scheduler: %s\n", err.Error())
	}

	fetchSecNum, _ := strconv.Atoi(fetchSec)

	// add a job to the scheduler
	j, err := s.NewJob(
		gocron.DurationJob(
			time.Duration(fetchSecNum)*time.Second,
		),
		gocron.NewTask(
			func(a string, b int) {
				// Request the HTML page.
				log.Println("Fetching FnP page")
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
					//log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
					log.Printf("Http error: %d\n", res.StatusCode)
				}

				// Load the HTML document
				doc, err := goquery.NewDocumentFromReader(res.Body)
				if err != nil {
					log.Printf("Error in scheduler: %s", err.Error())
				}

				// Find the review items
				doc.Find("body > main > article > div > section.panelV2.torrent-search__results > div > table > tbody > tr").Each(func(i int, s *goquery.Selection) {
					// For each item found, get the title
					torrentIdStr, _ := s.Attr("data-torrent-id")
					torrentId, _ := strconv.Atoi(torrentIdStr)
					categoryIdStr, _ := s.Attr("data-category-id")
					categoryId, _ := strconv.Atoi(categoryIdStr)
					typeIdStr, _ := s.Attr("data-type-id")
					typeId, _ := strconv.Atoi(typeIdStr)
					url := fmt.Sprintf("https://fearnopeer.com/torrents/%d", torrentId)
					title := strings.TrimSpace(s.Find("a.torrent-search--list__name").Text())
					uploader := strings.TrimSpace(s.Find("a.user-tag__link").Text())
					if strings.Contains(uploader, "(Anonymous)") {
						// remove parenthesis for anonymous uploader
						uploader = "Anonymous"
					}

					size := strings.TrimSpace(s.Find("td.torrent-search--list__size").Text())
					announceLine := fmt.Sprintf("Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]", getCategoryFriendlyStr(categoryId), getTypeFriendlyStr(typeId), title, size, uploader, url)
					announceDoc := Announce{TorrentId: torrentId, Name: title, Size: size, Category: getCategoryFriendlyStr(categoryId),
						Type: getTypeFriendlyStr(typeId), Uploader: uploader, URL: url, RawLine: announceLine, CreatedDate: time.Now()}

					_, err := mongoColl.InsertOne(context.TODO(), announceDoc)
					if err == nil {
						// announce to DB as not yet inserted in database
						log.Println(announceLine + "\n")
						irc.Msg(ircChannel, announceLine)
						//fmt.Printf("Inserted document with _id: %v\n", res.InsertedID)
					}

				})
			},
			"hello",
			1,
		),
	)
	if err != nil {
		log.Println("Error in job: " + err.Error())
	}
	// each job has a unique id
	log.Println("Cron Job id:" + j.ID().String())

	// start the scheduler
	go s.Start()

	log.Println("Press CTRL+C to exit")

	// Start up bot (this blocks until we disconnect)
	irc.Run()
	//select {} // block forever
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
var sayJoinMsg = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return strings.Contains(m.Raw, ircChannel+" :End of /NAMES list.") && strings.Contains(m.From, ircDomainTrigger)
		//return m.Command == "PRIVMSG" && m.Content == "-info"
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		irc.Msg(ircChannel, fmt.Sprintf("Hello %s!! Will start announcing soon...", ircChannel))
		return false
	},
}

// This trigger replies Hello when you say hello
var nickServAuth = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return strings.Contains(m.Raw, " :End of message of the day.") && strings.Contains(m.From, ircDomainTrigger)
		//return m.Command == "PRIVMSG" && m.Content == "-info"
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		irc.Msg("NickServ", "identify "+ircPassword)
		return false
	},
}
