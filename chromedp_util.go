package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var blockedUrls = [...]string{
	"*.css*",
	"*bootstrap-autofill-overlay.js*",
	"*alpine.js*",
	"*livewire.js*",
	"*unit3d.js*",
	"*virtual-select.js*",
}

func reloadChatPage(ctx context.Context, roomId, logLine string) {
	interruptWSPong.Flag()
	refreshedPage.Flag()
	chatVisibilityTasks := getChatVisibilityBrowserTask(roomId)
	log.Println(logLine)
	go chromedp.RunResponse(ctx,
		network.Enable(),
		network.SetBlockedURLS(blockedUrls[:]),
		chromedp.Reload(),
		chatVisibilityTasks)
}

// login to the webpage and click system chat box
func loginAndNavigate(url, username, password, roomId, totpKey string) chromedp.Tasks {
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
		network.SetBlockedURLS(blockedUrls[:]),
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

	chatVisibilityTasks := getChatVisibilityBrowserTask(roomId)

	if len(totpKey) > 0 {
		// totp login
		return append(loginTasks, totpTasks, chatVisibilityTasks, cookieTasks)
	}
	// totp-less login
	return append(loginTasks, chatVisibilityTasks, cookieTasks)
}

func getChatVisibilityBrowserTask(roomId string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Evaluate("document.querySelectorAll('svg').forEach(e => e.remove());document.querySelector('#frame > div > div.messages > ul')", nil), // remove svg animations, lowers cpu
		// wait for chat to be visible
		chromedp.WaitVisible(`//*[@id="chatbody"]`, chromedp.BySearch),
		chromedp.Sleep(5 * time.Second),
		chromedp.Click(fmt.Sprintf(`#frameTabs > div:nth-child(1) > ul > li:nth-child(%s) > a`, roomId), chromedp.ByQuery),
		chromedp.Sleep(5 * time.Second),
		chromedp.Click(fmt.Sprintf(`#frameTabs > div:nth-child(1) > ul > li:nth-child(%s) > a`, roomId), chromedp.ByQuery),
	}
}
