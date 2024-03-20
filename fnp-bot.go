package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pquerna/otp/totp"
	hbot "github.com/whyrusleeping/hellabot"
	logi "gopkg.in/inconshreveable/log15.v2"
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

var wsWatchdogTimerInitVal = 35 // 35 seconds
var wsWatchdogTimer = wsWatchdogTimerInitVal

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
		//chromedp.Flag("headless", false),
	)

	// Prepare browser context
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	go startBrowser(ctx, irc)

	go wsWatchdog(ctx)
	log.Println("---- Press CTRL+C to exit ----")

	// Start up bot (this blocks until we disconnect)
	irc.Run()
}

func wsWatchdog(ctx context.Context) {
	log.Println("Starting WS Watchdog")
	for wsWatchdogTimer > 0 {
		// poll and check every 1 sec if ws is already established
		time.Sleep(1 * time.Second)
		wsWatchdogTimer = wsWatchdogTimer - 1
		if pingPongWatchdog.Get() >= 1 {
			// ws Got Ponged
			pingPongWatchdog.Reset()
			wsWatchdogTimer = wsWatchdogTimerInitVal
			return
		}
	}
	refreshedPage.Set(1)
	log.Println("WS PingPong failed, reloading page")
	go chromedp.RunResponse(ctx, network.Enable(), chromedp.Reload())
	log.Println("Stopped WS Watchdog, should reset in a few moments")
	wsWatchdog(ctx)
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
	refreshedPage.Set(1)
	log.Println("WS not acknowledged, reloading page")
	go chromedp.RunResponse(ctx, network.Enable(), chromedp.Reload())
}

// Start and create browser
func startBrowser(ctx context.Context, irc *hbot.Bot) {
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
				go processAnnounce(p, irc, lastItemId, p.parseAnnounce, formatAnnounceStr)
			} else if announceType == FEATURE_ANNOUNCE {
				go processAnnounce(p, irc, lastFeatId, p.parseSparseAnnounce, formatFeatureStr)
			} else if announceType == FREELEECH_ANNOUNCE {
				go processAnnounce(p, irc, lastFLId, p.parseSparseAnnounce, formatFreeleechStr)
			}
		case *network.EventWebSocketFrameError:
		case *network.EventWebSocketClosed:
			refreshedPage.Set(1)
			log.Println("WS closed, reloading page")
			go chromedp.RunResponse(ctx, network.Enable(), chromedp.Reload())
		}
	})
	if err := chromedp.Run(ctx, loginAndNavigate(fetchSiteBaseUrl, siteUsername, sitePassword, totpToken)); err != nil {
		log.Fatalf("could not start chromedp: %v\n", err)
	}
	<-gotException
}

func processAnnounce(p *WebsocketMessage, irc *hbot.Bot, itemId *ItemIdCtr, parserFn ParserFunc, formatFn FormatterFunc) {
	a := parserFn(fetchSiteBaseUrl, siteApiKey)
	announceString := formatFn(a)
	if announceString != "" {
		log.Printf("Announcing to IRC: %v\n", announceString)
		go irc.Msg(ircChannel, announceString)
	}
	itemId.Set(a.Id)
}

// Checks for missed announce items
func performManualFetch(irc *hbot.Bot) {
	log.Println("Checking for missed items")
	tautology := func(item PageItem) bool { return true }
	if lastItemId.Get() != -1 {
		go fetchTorPage(cookieJar.Get(), "", lastItemId, tautology, irc)
	} else {
		log.Println("No manual fetch for uploads necessary")
	}
	if lastFeatId.Get() != -1 {
		go fetchTorPage(cookieJar.Get(), "&featured=true", lastFeatId, tautology, irc)
	} else {
		log.Println("No manual fetch for featuring items necessary")
	}
	if lastFLId.Get() != -1 {
		go fetchTorPage(cookieJar.Get(), "&free[0]=100", lastFLId,
			func(item PageItem) bool {
				return !item.Featured
			},
			irc)
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
	log.Printf("UNIT3D Bot name: %s\n", unit3dBotName)
	log.Printf("UNIT3D Room id: %s\n", roomId)
	log.Printf("Site Username: %s\n", siteUsername)
	log.Printf("Site Password: %s\n", "*******") // masked for safety
	log.Printf("Site API Key: %s\n", "*******")  // masked for safety
	log.Printf("TOTP Token: %s\n", "*******")    // masked for safety
}

func createIRCBot() *hbot.Bot {
	enableSaslBool, _ := strconv.ParseBool(enableSasl)
	enableSSLBool, _ := strconv.ParseBool(enableSSL)
	botConfig := func(bot *hbot.Bot) {
		bot.SSL = enableSSLBool
		bot.SASL = enableSaslBool
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

// login to the webpage and click system chat box
func loginAndNavigate(url, username, password, totpKey string) chromedp.Tasks {
	// retrieve cookies
	cookieTasks := chromedp.Tasks{chromedp.ActionFunc(func(ctx context.Context) error {
		cookies, err := network.GetCookies().Do(ctx)
		c := make([]string, len(cookies))
		for i, v := range cookies {
			aCookie := fmt.Sprintf("%s=%s", v.Name, v.Value)
			c[i] = aCookie
		}
		cookieJar.Set(strings.Join(c, ";"))
		if err != nil {
			return err
		}
		return nil
	}),
	}

	// login to the site using username and password
	loginTasks := chromedp.Tasks{
		network.Enable(),
		chromedp.Navigate(url),
		chromedp.Sleep(2 * time.Second),

		// wait for login form to be visible
		chromedp.WaitVisible(`//*[@class="auth-form__form"]`, chromedp.BySearch),

		chromedp.Click(`//*[@id="remember"]`, chromedp.BySearch),
		chromedp.SetValue(`//*[@id="username"]`, username, chromedp.BySearch),
		chromedp.Sleep(1 * time.Second),

		chromedp.SetValue(`//*[@id="password"]`, password, chromedp.BySearch),
		chromedp.Sleep(1 * time.Second),

		// login
		chromedp.Click(`//*[@class="auth-form__primary-button"]`, chromedp.BySearch),
		chromedp.Sleep(2 * time.Second),
	}

	// enter totp
	totpTasks := chromedp.Tasks{
		// wait for totp form to be visible and enter totp
		chromedp.WaitVisible(`//*[@class="auth-form__form"]`, chromedp.BySearch),
		chromedp.SetValue(`//*[@id="code"]`, getOtpKey(totpKey), chromedp.BySearch),
		chromedp.Sleep(1 * time.Second),
		chromedp.Click(`//*[@class="auth-form__primary-button"]`, chromedp.BySearch),
		chromedp.Sleep(2 * time.Second),
	}

	chatVisibilityTasks := chromedp.Tasks{
		// wait for chat to be visible
		chromedp.WaitVisible(`//*[@id="chatbody"]`, chromedp.BySearch),
		chromedp.Click(`#frameTabs > div:nth-child(1) > ul > li.panel__tab.panel__tab--active > a`, chromedp.ByQuery),
	}

	if len(totpKey) > 0 {
		// totp login
		return append(loginTasks, totpTasks, chatVisibilityTasks, cookieTasks)
	}
	// totp-less login
	return append(loginTasks, chatVisibilityTasks, cookieTasks)
}
