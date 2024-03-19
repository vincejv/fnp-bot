package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
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
var fetchSiteBaseUrl = getEnv("FETCH_BASE_URL", "https://site.com")
var enableSSL = getEnv("ENABLE_SSL", "True")
var enableSasl = getEnv("ENABLE_SASL", "False")
var announceLineFmt = getEnv("ANNOUNCE_LINE_FMT", "Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]")
var siteUsername = getEnv("SITE_USERNAME", "")
var sitePassword = getEnv("SITE_PASSWORD", "")
var totpToken = getEnv("SITE_TOTP_TOKEN", "")
var siteApiKey = getEnv("SITE_API_KEY", "")
var unit3dBotName = getEnv("SITE_BOT_NAME", "SystemBot")

func main() {
	flag.Parse()
	log.Print("Starting FNP Announcebot")
	logSettings()

	// Prepare IRC Bot
	irc := createIRCBot()

	log.Println("Starting browser")
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
	)

	// Prepare browser context
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	go startBrowser(ctx, irc)
	log.Println("---- Press CTRL+C to exit ----")

	// Start up bot (this blocks until we disconnect)
	irc.Run()
}

// Start and create browser
func startBrowser(ctx context.Context, irc *hbot.Bot) {
	gotException := make(chan bool, 1)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventWebSocketCreated:
			log.Printf("Established websocket connection")
		case *network.EventWebSocketFrameReceived:
			payload := ev.Response.PayloadData

			p := NewWebsocketParser()
			if err := p.Parse(payload); err != nil {
				log.Printf("could not parse websocket message: %v err: %v", payload, err)
				break
			}

			if !p.IsValid(unit3dBotName) {
				break
			}

			a := p.ParseAnnounce(fetchSiteBaseUrl, siteApiKey)
			announceString := formatAnnounceStr(a)

			log.Printf("Announcing to IRC: %v\n", announceString)

			if announceString != "" {
				irc.Msg(ircChannel, announceString)
			}
		case *network.EventWebSocketFrameError:
		case *network.EventWebSocketClosed:
			log.Println("WS closed, reloading page")
			chromedp.RunResponse(ctx, chromedp.Reload())
		}
	})
	if err := chromedp.Run(ctx, loginAndNavigate(fetchSiteBaseUrl, siteUsername, sitePassword, totpToken)); err != nil {
		log.Fatalf("could not start chromedp: %v\n", err)
	}
	<-gotException
}

func logSettings() {
	log.Println("Environment settings:")
	log.Printf("IRC Server: %s\n", serv)
	log.Printf("Bot nickname: %s\n", nick)
	log.Printf("IRC Announce channel: %s\n", ircChannel)
	log.Printf("IRC Password: %s\n", "*******")   // ircPassword masked for safety
	log.Printf("Crawler cookie: %s\n", "*******") // crawlerCookie masked for safety
	log.Printf("Enable SSL: %s\n", enableSSL)
	log.Printf("Enable SASL: %s\n", enableSasl)
	log.Printf("Site base url for fetching: %s\n", fetchSiteBaseUrl)
	log.Printf("Announce line format: %s\n", announceLineFmt)
	log.Printf("UNIT3D Bot name: %s\n", unit3dBotName)
	log.Printf("Site Username: %s\n", siteUsername)
	log.Printf("Site Password: %s\n", "*******") // masked for safety
	log.Printf("Site API Key: %s\n", "*******")  // masked for safety
	log.Printf("TOTP Token: %s\n", "*******")    // masked for safety
}

func formatAnnounceStr(announceLine *Announce) string {
	// Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]
	announceStr := fmt.Sprintf(announceLineFmt, announceLine.Category, announceLine.Type, announceLine.Release,
		announceLine.Size, announceLine.Uploader, announceLine.Url)
	return announceStr
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
	if len(totpKey) > 0 {
		// totp login
		return chromedp.Tasks{
			chromedp.Navigate(url),
			chromedp.Sleep(2 * time.Second),

			// wait for login form to be visible
			chromedp.WaitVisible(`//*[@class="auth-form__form"]`, chromedp.BySearch),

			chromedp.SetValue(`//*[@id="username"]`, username, chromedp.BySearch),
			chromedp.Sleep(1 * time.Second),

			chromedp.SetValue(`//*[@id="password"]`, password, chromedp.BySearch),
			chromedp.Sleep(1 * time.Second),

			// login
			chromedp.Click(`//*[@class="auth-form__primary-button"]`, chromedp.BySearch),
			chromedp.Sleep(2 * time.Second),

			// wait for totp form to be visible and enter totp
			chromedp.WaitVisible(`//*[@class="auth-form__form"]`, chromedp.BySearch),
			chromedp.SetValue(`//*[@id="code"]`, getOtpKey(totpKey), chromedp.BySearch),
			chromedp.Sleep(1 * time.Second),
			chromedp.Click(`//*[@class="auth-form__primary-button"]`, chromedp.BySearch),
			chromedp.Sleep(2 * time.Second),

			// wait for chat to be visible
			chromedp.WaitVisible(`//*[@id="chatbody"]`, chromedp.BySearch),
			chromedp.Click(`#frameTabs > div:nth-child(1) > ul > li.panel__tab.panel__tab--active > a`, chromedp.ByQuery),
		}
	} else {
		// totp-less login
		return chromedp.Tasks{
			chromedp.Navigate(url),
			chromedp.Sleep(2 * time.Second),

			// wait for login form to be visible
			chromedp.WaitVisible(`//*[@class="auth-form__form"]`, chromedp.BySearch),

			chromedp.SetValue(`//*[@id="username"]`, username, chromedp.BySearch),
			chromedp.Sleep(1 * time.Second),

			chromedp.SetValue(`//*[@id="password"]`, password, chromedp.BySearch),
			chromedp.Sleep(1 * time.Second),

			// login
			chromedp.Click(`//*[@class="auth-form__primary-button"]`, chromedp.BySearch),
			chromedp.Sleep(2 * time.Second),

			// wait for chat to be visible
			chromedp.WaitVisible(`//*[@id="chatbody"]`, chromedp.BySearch),
			chromedp.Click(`#frameTabs > div:nth-child(1) > ul > li.panel__tab.panel__tab--active > a`, chromedp.ByQuery),
		}
	}
}
