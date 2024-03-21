package main

import "fmt"

func formatUserMsgStr(announceLine *Announce) string {
	// Cat [%s] Type [%s] Name [%s] Size [%s] Uploader [%s] Url [%s]
	announceStr := fmt.Sprintf(userLineFmt, announceLine.Uploader, announceLine.RawLine)
	return announceStr
}
