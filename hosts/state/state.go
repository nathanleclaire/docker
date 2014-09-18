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

var states = []string{"Unknown", "Running", "Paused", "Saved", "Stopped"}

func (s State) String() string {
	if int(s) < len(states)-1 {
		return states[s]
	}
	return ""
}
