package ads

type Status string

const (
	// Experimenting Status = "experimenting"
	PendingStatus Status = "pending"
	RunningStatus Status = "running"
	StoppedStatus Status = "stopped"
)
