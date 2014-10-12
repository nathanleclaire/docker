package state

// State represents the state of a hosts
type State int

const (
	None State = iota
	Error
	Running
	Paused
	Saved
	Stopped
	Starting
)

var states = []string{
	"",
	"Error",
	"Running",
	"Paused",
	"Saved",
	"Stopped",
	"Starting",
}

func (s State) String() string {
	if int(s) < len(states)-1 {
		return states[s]
	}
	return ""
}
