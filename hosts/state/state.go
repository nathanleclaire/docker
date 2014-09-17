package state

// State represents the state of a hosts
type State int

const (
	Unknown State = iota
	Running
	Paused
	Saved
	Stopped
)
