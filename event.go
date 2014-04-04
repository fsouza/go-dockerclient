package docker

const (
	Create  = "create"
	Delete  = "delete"
	Destroy = "destroy"
	Die     = "die"
	Export  = "export"
	Kill    = "kill"
	Restart = "restart"
	Start   = "start"
	Stop    = "stop"
	Untag   = "untag"
)

type Event map[string]interface{}
