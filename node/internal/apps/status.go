package apps

type AppStatus int

const (
	STOPPED AppStatus = iota
	STARTING
	RUNNING
	DEAD
)

func (s AppStatus) String() string {
	switch s {
	case STOPPED:
		return "STOPPED"
	case STARTING:
		return "STARTING"
	case RUNNING:
		return "RUNNING"
	case DEAD:
		return "DEAD"
	default:
		panic("unknown app status")
	}
}
