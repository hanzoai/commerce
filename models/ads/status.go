package ads

type Status string

const (
	// Experimenting Status = "experimenting"
	Pending Status = "pending"
	Running Status = "running"
	Stopped Status = "stopped"
)
