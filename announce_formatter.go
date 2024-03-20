package main

import "fmt"

func formatAnnounceStr(announceLine *Announce) string {
	// Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]
	announceStr := fmt.Sprintf(announceLineFmt, announceLine.Category, announceLine.Type, announceLine.Release,
		announceLine.Size, announceLine.Uploader, announceLine.Url)
	return announceStr
}

func formatFeatureStr(announceLine *Announce) string {
	// Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]
	announceStr := fmt.Sprintf(featureLineFmt, announceLine.Category, announceLine.Type, announceLine.Release,
		announceLine.Size, announceLine.Uploader, announceLine.Url)
	return announceStr
}

func formatFreeleechStr(announceLine *Announce) string {
	// Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]
	announceStr := fmt.Sprintf(freeleechLineFmt, announceLine.Category, announceLine.Type, announceLine.Release,
		announceLine.Size, announceLine.Uploader, announceLine.Url)
	return announceStr
}
