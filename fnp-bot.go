package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pquerna/otp/totp"
)

const LAST_ANNOUNCE_SETTING_ID int = 1

var serv = os.Getenv("IRC_SERVER")
var nick = os.Getenv("BOT_NICK")
var ircChannel = os.Getenv("IRC_CHANNEL")
var ircPassword = os.Getenv("IRC_BOT_PASSWORD")

// var ircDomainTrigger = os.Getenv("IRC_DOMAIN_TRIGGER") // Trigger for hello message and nickserv auth
var fetchNoItems = getEnv("FETCH_NO_OF_ITEMS", "25") // For manual fetching
var fetchSiteBaseUrl = getEnv("FETCH_BASE_URL", "https://site.com")
var enableSSL = getEnv("ENABLE_SSL", "True")
var enableSasl = getEnv("ENABLE_SASL", "False")
var announceLineFmt = getEnv("ANNOUNCE_LINE_FMT", "Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]")
var featureLineFmt = getEnv("FEATURE_LINE_FMT", "NOW FEATURING!! Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]")
var freeleechLineFmt = getEnv("FREELEECH_LINE_FMT", "FREELEECH TORRENT!! Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]")
var siteUsername = getEnv("SITE_USERNAME", "")
var sitePassword = getEnv("SITE_PASSWORD", "")
var totpToken = getEnv("SITE_TOTP_TOKEN", "")
var siteApiKey = getEnv("SITE_API_KEY", "")
var unit3dBotName = getEnv("SITE_BOT_NAME", "SystemBot")
var roomId = getEnv("ROOM_ID", "2")

type FormatterFunc func(*Announce) string

func main() {
	flag.Parse()
	log.Print("Starting FNP Announcebot")
	logSettings()
	initMutex()

	// Prepare IRC Bot
	irc := createIRCBot()

	log.Println("Starting browser")
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("disable-domain-reliability", true),
		chromedp.Flag("disable-component-update", true),
		//chromedp.Flag("headless", false),
	)

	// Prepare browser context
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	go startBrowser(ctx, irc)

	go wsWatchdog(ctx, roomId)
	log.Println("---- Press CTRL+C to exit ----")

	// Start up bot (this blocks until we disconnect)
	irc.Loop()
}

func waitAndReload(ctx context.Context, timeout int) {
	for timeout > 0 {
		// poll and check every 1 sec if ws is already established
		time.Sleep(1 * time.Second)
		timeout = timeout - 1
		if wsHandshake.Get() != 0 {
			// ws NOW acknowledged, no need to refresh page
			return
		}
	}
	go reloadChatPage(ctx, roomId, "WS not acknowledged, reloading page")
}

// Start and create browser
func startBrowser(ctx context.Context, irc *ircevent.Connection) {
	gotException := make(chan bool, 1)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventWebSocketCreated:
			wsHandshake.Set(0)
			log.Printf("Page loaded, create websocket connection, waiting for handshake, timeout is 15s, reload if not acknowledged")
			if refreshedPage.Get() == 1 {
				// paged refresh due to WS Closing
				refreshedPage.Set(0)
				go performManualFetch(irc)
			} else {
				log.Println("Intial WS connection created")
			}
			go waitAndReload(ctx, 15)
		case *network.EventWebSocketHandshakeResponseReceived:
			wsHandshake.Set(1)
			log.Printf("Handshake acknowledged, connection established")
		case *network.EventWebSocketFrameReceived:
			payload := ev.Response.PayloadData
			p := NewWebsocketParser()

			rawMsg, _ := Decode(payload)
			if rawMsg.Type == MessageTypePong {
				log.Println("<---- WS PONG")
				pingPongWatchdog.Increment()
				break
			}

			if err := p.parseSocketMsg(payload, roomId); err != nil {
				log.Printf("could not parse websocket message: %v err: %v", payload, err)
				break
			}
			announceType := p.determineType(unit3dBotName)
			if announceType == UPLOAD_ANNOUNCE {
				go processAnnounce(irc, lastItemId, p.parseAnnounce, formatAnnounceStr)
			} else if announceType == FEATURE_ANNOUNCE {
				go processAnnounce(irc, lastFeatId, p.parseSparseAnnounce, formatFeatureStr)
			} else if announceType == FREELEECH_ANNOUNCE {
				go processAnnounce(irc, lastFLId, p.parseSparseAnnounce, formatFreeleechStr)
			}
		case *network.EventWebSocketFrameError:
		case *network.EventWebSocketClosed:
			go reloadChatPage(ctx, roomId, "WS closed/errored, reloading page")
		}
	})
	if err := chromedp.Run(ctx, loginAndNavigate(fetchSiteBaseUrl, siteUsername, sitePassword, roomId, totpToken)); err != nil {
		log.Fatalf("could not start chromedp: %v\n", err)
	}
	<-gotException
}

func processAnnounce(irc *ircevent.Connection, itemId *ItemIdCtr, parserFn ParserFunc, formatFn FormatterFunc) {
	a := parserFn(fetchSiteBaseUrl, siteApiKey)
	announceString := formatFn(a)
	if announceString != "" {
		log.Printf("Announcing to IRC: %v\n", announceString)
		go irc.Privmsg(ircChannel, announceString)
	}
	itemId.Set(a.Id)
}

// Checks for missed announce items
func performManualFetch(irc *ircevent.Connection) {
	log.Println("Checking for missed items")
	// only fetch 10 minute old items
	timeFilter := func(item PageItem) bool {
		thresh := time.Now().Add(-10 * time.Minute)
		return item.UploadedDate.After(thresh)
	}
	if lastItemId.Get() != -1 {
		go fetchTorPage(cookieJar.Get(), "", lastItemId, timeFilter, irc, announceLineFmt)
	} else {
		log.Println("No manual fetch for uploads necessary")
	}
	if lastFeatId.Get() != -1 {
		go fetchTorPage(cookieJar.Get(), "&featured=true", lastFeatId, timeFilter, irc, featureLineFmt)
	} else {
		log.Println("No manual fetch for featuring items necessary")
	}
	if lastFLId.Get() != -1 {
		go fetchTorPage(cookieJar.Get(), "&free[0]=100", lastFLId,
			func(item PageItem) bool {
				// only fetch 10 minute old items
				thresh := time.Now().Add(-10 * time.Minute)
				return !item.Featured && item.UploadedDate.After(thresh)
			},
			irc, freeleechLineFmt)
	} else {
		log.Println("No manual fetch for FL items necessary")
	}
}

func logSettings() {
	log.Println("Environment settings:")
	log.Printf("IRC Server: %s\n", serv)
	log.Printf("Bot nickname: %s\n", nick)
	log.Printf("IRC Announce channel: %s\n", ircChannel)
	log.Printf("IRC Password: %s\n", "*******") // ircPassword masked for safety
	log.Printf("Number of items to fetch on manual pull: %s\n", fetchNoItems)
	log.Printf("Enable SSL: %s\n", enableSSL)
	log.Printf("Enable SASL: %s\n", enableSasl)
	log.Printf("Site base url for fetching: %s\n", fetchSiteBaseUrl)
	log.Printf("Announce line format: %s\n", announceLineFmt)
	log.Printf("Featured Announce line format: %s\n", featureLineFmt)
	log.Printf("Freeleech Announce line format: %s\n", freeleechLineFmt)
	log.Printf("UNIT3D Bot name: %s\n", unit3dBotName)
	log.Printf("UNIT3D Room id: %s\n", roomId)
	log.Printf("Site Username: %s\n", siteUsername)
	log.Printf("Site Password: %s\n", "*******") // masked for safety
	log.Printf("Site API Key: %s\n", "*******")  // masked for safety
	log.Printf("TOTP Token: %s\n", "*******")    // masked for safety
}

func createIRCBot() *ircevent.Connection {
	enableSaslBool, _ := strconv.ParseBool(enableSasl)
	enableSSLBool, _ := strconv.ParseBool(enableSSL)
	serverPassword := ""
	saslPassword := ""
	if enableSaslBool {
		// if sasl is enabled, can't login with server password
		saslPassword = ircPassword
	} else {
		// if sasl disabled, treat ircPassword as server password
		serverPassword = ircPassword
	}
	irc := ircevent.Connection{
		Server:       serv,
		UseTLS:       enableSSLBool,
		UseSASL:      enableSaslBool,
		Password:     serverPassword,
		SASLLogin:    nick,
		SASLPassword: saslPassword,
		SASLOptional: true,
		Nick:         nick,
		Debug:        false,
		RequestCaps:  []string{"server-time", "message-tags"},
		Log:          log.Default(),
	}
	irc.AddConnectCallback(func(e ircmsg.Message) { irc.Join(ircChannel) })

	err := irc.Connect()
	if err != nil {
		log.Fatal(err)
	}
	return &irc
}

// Utility function for getting environment variables with default value
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getOtpKey(totpToken string) string {
	otp, err := totp.GenerateCode(totpToken, time.Now())
	if err != nil {
		log.Fatal("failed totp code")
	}
	return otp
}
