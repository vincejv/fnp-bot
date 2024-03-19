package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	hbot "github.com/whyrusleeping/hellabot"
)

type PageItem struct {
	TorrentId   int64
	Name        string
	Size        string
	CreatedDate time.Time
	Category    string
	Type        string
	Uploader    string
	URL         string
	RawLine     string
}

func fetchTorPage(cookie string, lastId *ItemIdCtr, irc *hbot.Bot) {
	// Request the HTML page.
	log.Println("Fetching possible missed items due to WS Closing")
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/torrents?perPage=%s", fetchSiteBaseUrl, fetchNoItems), nil)
	req.Header.Set("Cookie", cookie)
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

	var fetchedTors []PageItem

	// Scrape the items
	doc.Find("body > main > article > div > section.panelV2.torrent-search__results > div > table > tbody > tr").Each(func(i int, s *goquery.Selection) {

		torrentIdStr, _ := s.Attr("data-torrent-id")
		torrentId, _ := strconv.ParseInt(torrentIdStr, 10, 64)
		categoryIdStr, _ := s.Attr("data-category-id")
		categoryId, _ := strconv.Atoi(categoryIdStr)
		typeIdStr, _ := s.Attr("data-type-id")
		typeId, _ := strconv.Atoi(typeIdStr)
		url := fmt.Sprintf("%s/torrents/%d", fetchSiteBaseUrl, torrentId)
		title := strings.TrimSpace(s.Find("a.torrent-search--list__name").Text())
		uploader := strings.TrimSpace(s.Find("span.torrent-search--list__uploader").Text())

		// remove parenthesis for anonymous uploader
		if strings.Contains(uploader, "(Anonymous)") {
			uploader = "Anonymous"
		}

		size := strings.TrimSpace(s.Find("td.torrent-search--list__size").Text())
		// Category, type, name, size, uploader, url
		announceLine := fmt.Sprintf(announceLineFmt, getCategoryFriendlyStr(categoryId), getTypeFriendlyStr(typeId), title, size, uploader, url)

		// Store fetched torrents temporarily for processing
		announceDoc := PageItem{TorrentId: torrentId, Name: title, Size: size, Category: getCategoryFriendlyStr(categoryId),
			Type: getTypeFriendlyStr(typeId), Uploader: uploader, URL: url, RawLine: announceLine, CreatedDate: time.Now()}
		fetchedTors = append(fetchedTors, announceDoc)
	})

	// Must sort in ascending so next block will work correctly
	sort.Slice(fetchedTors, func(tori, torj int) bool {
		return fetchedTors[tori].TorrentId < fetchedTors[torj].TorrentId
	})

	// Examine fetched torrents and push to IRC based on last send item id
	for _, tor := range fetchedTors {
		if tor.TorrentId > lastId.Get() {
			log.Println("Missed item: " + tor.RawLine)
			go irc.Msg(ircChannel, tor.RawLine)
			lastId.Set(tor.TorrentId)
		}
	}
}

func getCategoryFriendlyStr(catId int) string {
	switch catId {
	case 1:
		return "Movies"
	case 2:
		return "TV"
	case 3:
		return "Music"
	case 6:
		return "Anime"
	case 4:
		return "Games"
	case 5:
		return "Apps"
	case 9:
		return "Sport"
	case 11:
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