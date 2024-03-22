package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/ergochat/irc-go/ircevent"
)

type PageItems struct {
	Data []PageItem `json:"data"`
}

type PageItem struct {
	ID   int64 `json:"id"`
	Bot  any   `json:"bot"`
	User struct {
		ID         int64  `json:"id"`
		Username   string `json:"username"`
		ChatStatus struct {
			ID        int       `json:"id"`
			Name      string    `json:"name"`
			Color     string    `json:"color"`
			Icon      string    `json:"icon"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"chat_status"`
		ChatStatusID int `json:"chat_status_id"`
		ChatroomID   int `json:"chatroom_id"`
		Group        struct {
			ID             int    `json:"id"`
			Name           string `json:"name"`
			Slug           string `json:"slug"`
			Position       int    `json:"position"`
			Level          int    `json:"level"`
			DownloadSlots  any    `json:"download_slots"`
			Color          string `json:"color"`
			Icon           string `json:"icon"`
			Effect         string `json:"effect"`
			IsInternal     int    `json:"is_internal"`
			IsEditor       int    `json:"is_editor"`
			IsOwner        int    `json:"is_owner"`
			IsAdmin        int    `json:"is_admin"`
			IsModo         int    `json:"is_modo"`
			IsTrusted      int    `json:"is_trusted"`
			IsImmune       int    `json:"is_immune"`
			IsFreeleech    int    `json:"is_freeleech"`
			IsDoubleUpload int    `json:"is_double_upload"`
			IsRefundable   int    `json:"is_refundable"`
			CanUpload      int    `json:"can_upload"`
			IsIncognito    int    `json:"is_incognito"`
			Autogroup      int    `json:"autogroup"`
		} `json:"group"`
		GroupID int    `json:"group_id"`
		Title   any    `json:"title"`
		Image   string `json:"image"`
	} `json:"user"`
	Receiver any `json:"receiver"`
	Chatroom struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	} `json:"chatroom"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FetchFilter func(PageItem) bool

// Manual fetch for fetching missed items when websocket is temporarily dropped or disconnected
func fetchTorPage(cookie, addtlQuery string, lastId *ItemIdCtr, filter FetchFilter, irc *ircevent.Connection, announceFmt string) {
	// Request the HTML page.
	time.Sleep(5 * time.Second) // artificially sleep by 5 seconds
	url := fmt.Sprintf("%s/api/chat/messages/%s%s", fetchSiteBaseUrl, roomId, addtlQuery)
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
		time.Sleep(5 * time.Second)
		fetchTorPage(cookie, addtlQuery, lastId, filter, irc, announceFmt)
		return
	}

	rawRes, _ := io.ReadAll(res.Body)
	fetchedItems, _ := decodeItems(rawRes)
	filteredItems := []PageItem{}

	for _, v := range fetchedItems.Data {
		if filter(v) {
			filteredItems = append(filteredItems, v)
		}
	}

	// Must sort in ascending so next block will work correctly
	sort.Slice(filteredItems, func(tori, torj int) bool {
		return filteredItems[tori].ID < filteredItems[torj].ID
	})

	// Examine fetched torrents and push to IRC based on last send item id
	for _, tor := range filteredItems {
		if tor.ID > lastId.Get() {
			cleanLine := fmt.Sprintf(userLineFmt, tor.User.Username, cleanHTML(tor.Message))
			log.Println("Missed chat: " + cleanLine)
			irc.Privmsg(ircChannel, cleanLine) // send in order
			lastId.Set(tor.ID)
		}
	}
}

// Function to decode JSON data into a struct
func decodeItems(data []byte) (*PageItems, error) {
	var resp PageItems
	err := json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
