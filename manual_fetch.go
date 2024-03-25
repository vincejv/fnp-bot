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
	"github.com/ergochat/irc-go/ircevent"
)

type PageItem struct {
	TorrentId    int64
	Name         string
	Size         string
	CreatedDate  time.Time
	UploadedDate time.Time
	Category     string
	Type         string
	Uploader     string
	URL          string
	Featured     bool
	RawLine      string
}

type FetchFilter func(PageItem) bool

// Manual fetch for fetching missed items when websocket is temporarily dropped or disconnected
func fetchTorPage(cookie, addtlQuery string, lastId *ItemIdCtr, filter FetchFilter, irc *ircevent.Connection, announceFmt string) {
	// Request the HTML page.
	time.Sleep(15 * time.Second) // artificially sleep by 5 seconds
	url := fmt.Sprintf("%s/torrents?perPage=%s%s", fetchSiteBaseUrl, fetchNoItems, addtlQuery)
	log.Println("Fetching possible missed items due to WS Closing")
	log.Println(url)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
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
		log.Printf("Http error: %d, retrying\n", res.StatusCode)
		time.Sleep(10 * time.Second)
		fetchTorPage(cookie, addtlQuery, lastId, filter, irc, announceFmt)
		return
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("Error in scheduler: %s", err.Error())
	}

	var fetchedTors []PageItem
	var filteredIds []int64

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
		layout := "2006-01-02 15:04:05"
		uploadTimeStr := s.Find("td.torrent-search--list__age time").AttrOr("datetime", layout)
		uploadTime, _ := time.Parse(layout, uploadTimeStr)
		featured := false
		if s.Find("i.torrent-icons__featured").Length() > 0 {
			featured = true
		}

		// remove parenthesis for anonymous uploader
		if strings.Contains(uploader, "(Anonymous)") {
			uploader = "Anonymous"
		}

		size := strings.TrimSpace(s.Find("td.torrent-search--list__size").Text())
		// Category, type, name, size, uploader, url
		announceLine := fmt.Sprintf(announceFmt, getCategoryFriendlyStr(categoryId), getTypeFriendlyStr(typeId), title, size, uploader, url)

		// Store fetched torrents temporarily for processing
		announceDoc := PageItem{TorrentId: torrentId, Name: title, Size: size, Category: getCategoryFriendlyStr(categoryId),
			Type: getTypeFriendlyStr(typeId), Uploader: uploader, URL: url, Featured: featured, RawLine: announceLine, CreatedDate: time.Now().UTC(),
			UploadedDate: uploadTime}
		if filter(announceDoc) {
			fetchedTors = append(fetchedTors, announceDoc)
		} else {
			filteredIds = append(filteredIds, announceDoc.TorrentId)
		}
	})
	log.Printf("Not announcing, items doesn't pass filter: %v\n", filteredIds)

	// Must sort in ascending so next block will work correctly
	sort.Slice(fetchedTors, func(tori, torj int) bool {
		return fetchedTors[tori].TorrentId < fetchedTors[torj].TorrentId
	})

	var skippedIds []int64
	// Examine fetched torrents and push to IRC based on last send item id
	for _, tor := range fetchedTors {
		if tor.TorrentId > lastId.Get() {
			log.Println("Missed item: " + tor.RawLine)
			go irc.Privmsg(ircChannel, tor.RawLine)
			lastId.Set(tor.TorrentId)
		} else {
			skippedIds = append(skippedIds, tor.TorrentId)
		}
	}
	log.Printf("Not announcing, item already announced: %v", skippedIds)
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
