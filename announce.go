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
	RawLine      string
}

func (a Announce) String() string {
	return fmt.Sprintf("New torrent: '%v' Uploader: '%v' - %v", a.Release, a.Uploader, a.Url)
}
