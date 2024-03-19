package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type WebsocketMessage struct {
	Message struct {
		Bot struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Command     string `json:"command"`
			IsSystembot int    `json:"is_systembot"`
		} `json:"bot"`
		User struct {
			Username string `json:"username"`
			Group    struct {
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"group"`
		} `json:"user"`
		Message string `json:"message"`
	} `json:"message"`
}

type TorrentDetail struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		Meta struct {
			Poster string `json:"poster"`
			Genres string `json:"genres"`
		} `json:"meta"`
		Name           string    `json:"name"`
		ReleaseYear    any       `json:"release_year"`
		Category       string    `json:"category"`
		Type           string    `json:"type"`
		Resolution     string    `json:"resolution"`
		MediaInfo      string    `json:"media_info"`
		BdInfo         any       `json:"bd_info"`
		Description    string    `json:"description"`
		InfoHash       string    `json:"info_hash"`
		Size           int       `json:"size"`
		NumFile        int       `json:"num_file"`
		Freeleech      string    `json:"freeleech"`
		DoubleUpload   bool      `json:"double_upload"`
		Internal       int       `json:"internal"`
		Uploader       string    `json:"uploader"`
		Seeders        int       `json:"seeders"`
		Leechers       int       `json:"leechers"`
		TimesCompleted int       `json:"times_completed"`
		TmdbID         int       `json:"tmdb_id"`
		ImdbID         int       `json:"imdb_id"`
		TvdbID         int       `json:"tvdb_id"`
		MalID          int       `json:"mal_id"`
		IgdbID         int       `json:"igdb_id"`
		CategoryID     int       `json:"category_id"`
		TypeID         int       `json:"type_id"`
		ResolutionID   int       `json:"resolution_id"`
		CreatedAt      time.Time `json:"created_at"`
		DownloadLink   string    `json:"download_link"`
		DetailsLink    string    `json:"details_link"`
	} `json:"attributes"`
}

func NewWebsocketParser() *WebsocketMessage {
	return &WebsocketMessage{}
}

func (m *WebsocketMessage) Parse(payload string) error {
	// decode socketio message
	decoded, err := Decode(payload)
	if err != nil {
		return err
	}

	if decoded.Method != "new.message" {
		return nil
	}

	room := strings.Contains(decoded.Args, "presence-chatroom.2")
	if !room {
		return nil
	}

	data := decoded.Args

	// remove room from string
	cleanJson := strings.TrimLeft(data, `"presence-chatroom.2",`)

	// marshal json to struct
	if err = json.Unmarshal([]byte(cleanJson), &m); err != nil {
		log.Printf("could not unmarshal to struct: %v err: %v\n", cleanJson, err)
		return err
	}

	return nil
}

func (m *WebsocketMessage) IsBotName(name string) bool {
	// check for correct user
	if m.Message.Bot.Name == name {
		return true
	}
	return false
}

func (m *WebsocketMessage) IsSubtitle() bool {
	if strings.Contains(m.Message.Message, "subtitle for ") {
		return true
	}
	return false
}

func (m *WebsocketMessage) IsNewUpload() bool {
	if strings.Contains(m.Message.Message, "has uploaded") {
		return true
	}
	return false
}

func (m *WebsocketMessage) IsValid(botname string) bool {
	if !m.IsBotName(botname) {
		return false
	}

	if !m.IsNewUpload() {
		return false
	}

	if m.IsSubtitle() {
		return false
	}

	return true
}

func (m *WebsocketMessage) parseUploader() string {

	// uploader
	var uploader string
	anonUploader := strings.Contains(m.Message.Message, "An anonymous user has uploaded")
	if anonUploader {
		uploader = "anonymous"
	} else {
		// split string and grab idx [1]
		//ul := strings.Split(*anMsg, "")
		//uploader = ul[1]

		uploaderRegex := regexp.MustCompile("<a[^>]+href=\\\"https?\\:\\/\\/[^\\/]+\\/users\\/\\w+\\\"[^>]*>(.*?)<\\/a>")
		uploaderMatches := uploaderRegex.FindStringSubmatch(m.Message.Message)
		uploader = uploaderMatches[1]
	}

	return uploader
}

func (m *WebsocketMessage) parseCategory() string {
	category := ""

	re := regexp.MustCompile(`(?mi)has uploaded a new (.+)\. (.*), grab it now!`)
	matches := re.FindStringSubmatch(m.Message.Message)

	if len(matches) >= 1 {
		category = matches[1]
	}

	return category
}

func (m *WebsocketMessage) parseRelease() (url string, rel string) {

	// url
	// matches into two groups - url and torrent name
	urlNameRegex := regexp.MustCompile("<a[^>]+href=\\\"(https?\\:\\/\\/[^\\/]+\\/torrents\\/\\d+)\\\"[^>]*>(.*?)<\\/a>")
	matches := urlNameRegex.FindStringSubmatch(m.Message.Message)

	url = matches[1]
	rel = matches[2]

	return
}

func (m *WebsocketMessage) parseTorrentId(url string) int {
	urlRegx := regexp.MustCompile(`(https?\:\/\/.*?\/).*\/(\d+)`)
	matches := urlRegx.FindStringSubmatch(url)
	tId, _ := strconv.Atoi(matches[2])
	return tId
}

func (m *WebsocketMessage) ParseAnnounce(baseUrl, apiKey string) *Announce {
	return m.parseAnnounce(baseUrl, apiKey)
}

func (m *WebsocketMessage) parseAnnounce(baseUrl, apiKey string) *Announce {
	a := &Announce{}

	a.Uploader = m.parseUploader()
	a.Category = m.parseCategory()

	a.Url, a.Release = m.parseRelease()
	tDtl := getTorrentDtl(baseUrl, apiKey, m.parseTorrentId(a.Url))
	a.Size = ByteCountIEC(int64(tDtl.Attributes.Size))
	a.Type = tDtl.Attributes.Type
	if tDtl.Attributes.DoubleUpload {
		a.DoubleUpload = "Yes"
	} else {
		a.DoubleUpload = "No"
	}
	a.Freeleech = tDtl.Attributes.Freeleech
	if tDtl.Attributes.Internal == 0 {
		a.Internal = "No"
	} else {
		a.Internal = "Yes"
	}

	return a
}

func Parse(payload string) (string, error) {

	msg, err := parse(payload)
	if err != nil {
		return "", err
	}

	// botName := "Xenomorph-XX121"

	// // check for correct user
	// if msg.Message.Bot.Name != botName {
	// 	return "", nil
	// }

	anMsg := &msg.Message.Message

	// check if message contains "has uploaded"
	isNewUpload := strings.Contains(*anMsg, "has uploaded")
	if !isNewUpload {
		return "", nil
	}

	isSubtitles := strings.Contains(*anMsg, "subtitle for ")
	if isSubtitles {
		return "", nil
	}

	// uploader
	var uploader string
	anonUploader := strings.Contains(*anMsg, "An anonymous user has uploaded")
	if anonUploader {
		uploader = "anonymous"
	} else {
		// split string and grab idx [1]
		//ul := strings.Split(*anMsg, "")
		//uploader = ul[1]

		uploaderRegex := regexp.MustCompile("<a[^>]+href=\\\"https?\\:\\/\\/[^\\/]+\\/users\\/\\w+\\\"[^>]*>(.*?)<\\/a>")
		uploaderMatches := uploaderRegex.FindStringSubmatch(*anMsg)
		uploader = uploaderMatches[1]
	}

	// url
	// matches into two groups - url and torrent name
	urlNameRegex := regexp.MustCompile("<a[^>]+href=\\\"(https?\\:\\/\\/[^\\/]+\\/torrents\\/\\d+)\\\"[^>]*>(.*?)<\\/a>")
	matches := urlNameRegex.FindStringSubmatch(*anMsg)

	torrentUrl := matches[1]
	torrentName := matches[2]

	announceMsg := ""
	announceMsg = fmt.Sprintf("New torrent: '%v' Uploader: '%v' - %v", torrentName, uploader, torrentUrl)
	log.Printf("announce: %+v\n", announceMsg)

	return announceMsg, nil
}

func getTorrentDtl(baseUrl, apiKey string, tid int) *TorrentDetail {
	torDtl := new(TorrentDetail)
	resp, err := http.Get(fmt.Sprintf("%s/api/torrents/%d?api_token=%s", baseUrl, tid, apiKey))
	if err != nil {
		log.Printf("Unable to fetch torrent %+v\n", tid)
	}
	// Read and unmarshal the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Unable to read response body %+v\n", err)
		return nil
	}
	err = json.Unmarshal(body, &torDtl)
	if err != nil {
		log.Printf("Unable to unmarshall response %+v\n", err)
		return nil
	}
	defer resp.Body.Close()
	return torDtl
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func parse(payload string) (*WebsocketMessage, error) {
	// decode socketio message
	decoded, err := Decode(payload)
	if err != nil {
		return nil, err
	}

	if decoded.Method != "new.message" {
		return nil, nil
	}

	room := strings.Contains(decoded.Args, "presence-chatroom.2")
	if !room {
		return nil, nil
	}

	data := decoded.Args

	// remove room from string
	cleanJson := strings.TrimLeft(data, `"presence-chatroom.2",`)

	// marshal json to struct
	var msg WebsocketMessage
	if err = json.Unmarshal([]byte(cleanJson), &msg); err != nil {
		log.Printf("could not unmarshal to struct: %v err: %v\n", cleanJson, err)
		return nil, err
	}

	return &msg, nil
}
