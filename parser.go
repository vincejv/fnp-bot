package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"time"
)

// Announce types
const (
	UPLOAD_ANNOUNCE = iota
	FEATURE_ANNOUNCE
	OTHER_ANNOUNCE
	IGNORE_ANNOUNCE
	FREELEECH_ANNOUNCE
	USER_MESSAGE
)

// Parser function types
type ParserFunc func(string, string) *Announce

type WebsocketMessage struct {
	Message struct {
		ID  int64 `json:"id"`
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

// Parses websocket message
func (m *WebsocketMessage) parseSocketMsg(payload, roomId string) error {
	// decode socketio message
	decoded, err := Decode(payload)
	if err != nil {
		return err
	}

	if decoded.Method != "new.message" {
		return nil
	}

	room := strings.Contains(decoded.Args, fmt.Sprintf("presence-chatroom.%s", roomId))
	if !room {
		return nil
	}

	data := decoded.Args

	// remove room from string
	cleanJson := strings.TrimLeft(data, fmt.Sprintf(`"presence-chatroom.%s",`, roomId))

	// marshal json to struct
	if err = json.Unmarshal([]byte(cleanJson), &m); err != nil {
		log.Printf("could not unmarshal to struct: %v err: %v\n", cleanJson, err)
		return err
	}

	return nil
}

func (m *WebsocketMessage) isBotName(name string) bool {
	// check for correct user
	return m.Message.Bot.Name == name || m.Message.User.Username == "System" || m.Message.User.Username == unit3dBotName
}

func (m *WebsocketMessage) isSubtitle() bool {
	return strings.Contains(m.Message.Message, "subtitle for ")
}

func (m *WebsocketMessage) isNewUpload() bool {
	return strings.Contains(m.Message.Message, "has uploaded")
}

func (m *WebsocketMessage) isFeaturedAnnounce() bool {
	return strings.Contains(m.Message.Message, "has been added to the Featured Torrents Slider")
}

func (m *WebsocketMessage) isFreeleechAnnounce() bool {
	return strings.Contains(m.Message.Message, "has been granted 100% FreeLeech")
}

// Determine announce type
func (m *WebsocketMessage) determineType(botname string) int {
	if !m.isBotName(botname) {
		return USER_MESSAGE // ignore chat messages not from Bot user
	}

	if m.isFeaturedAnnounce() {
		return FEATURE_ANNOUNCE
	}

	if m.isFreeleechAnnounce() {
		return FREELEECH_ANNOUNCE
	}

	if !m.isNewUpload() {
		return IGNORE_ANNOUNCE // not a new upload
	}

	if m.isSubtitle() {
		return IGNORE_ANNOUNCE
	}

	return UPLOAD_ANNOUNCE
}

func cleanHTML(input string) string {
	// Unescape HTML entities
	input = html.UnescapeString(input)

	// Regular expression to match both HTML tags and BBCode
	re := regexp.MustCompile(`\[.*?\]|\[\/.*?\]|<[^>]*>`)

	// Replace HTML tags and BBCode with empty string
	output := re.ReplaceAllStringFunc(input, func(match string) string {
		// Check if it's an image tag
		if strings.HasPrefix(match, "<img ") {
			// Extract alt text from the image tag
			altText := extractAltText(match)
			return altText
		}
		// For other tags, just replace with empty string
		return ""
	})

	return output
}

func extractAltText(imgTag string) string {
	// Regular expression to extract alt text from the image tag
	re := regexp.MustCompile(`alt="(.*?)"`)
	match := re.FindStringSubmatch(imgTag)
	if len(match) >= 2 {
		// If alt text is found, return it
		return match[1]
	}
	// If alt text is not found, return empty string
	return ""
}

// Parse regular announce, that contains uploader and categories in the announce message itself
func (m *WebsocketMessage) parseUserMessage(baseUrl, apiKey string) *Announce {
	a := &Announce{}

	a.Id = m.Message.ID
	a.Uploader = m.Message.User.Username
	a.RawLine = cleanHTML(m.Message.Message)

	return a
}
