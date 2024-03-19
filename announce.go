package main

import (
	"fmt"
)

type Announce struct {
	Id           int64
	Url          string
	Release      string
	Uploader     string
	Category     string
	Size         string
	Type         string
	Freeleech    string
	DoubleUpload string
	Internal     string
}

func (a Announce) String() string {
	return fmt.Sprintf("New torrent: '%v' Uploader: '%v' - %v", a.Release, a.Uploader, a.Url)
}

// func (a Announce) FormattedString(format string) string {

// 	// setup text template to inject variables into
// 	tmpl, err := template.New("announce").Parse(format)
// 	if err != nil {
// 		log.r(err).Msg("could not parse announce format template")
// 		return ""
// 	}

// 	var b bytes.Buffer
// 	err = tmpl.Execute(&b, &a)
// 	if err != nil {
// 		log.Error().Err(err).Msg("could not execute announce format template output")
// 		return ""
// 	}

// 	log.Trace().Msg("announce formatted")

// 	return b.String()
// }
