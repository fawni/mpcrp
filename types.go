package main

type state int8

type playback struct {
	file           string
	state          state
	statestring    string
	position       int
	duration       int
	durationstring string
	version        string
	// filepatharg string
	// filepath    string
	// filedirarg  string
	// filedir     string
	// positionstring string
	// volumelevel    int
	// muted          bool
	// playbackrate   float32
	// size           string
	// reloadtime     int
}

const (
	idling state = iota - 1
	stopped
	paused
	playing
)
