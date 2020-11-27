package docker

// Filter Arguments
// Use NewArgs() and Add(key, val) to populate
type Args map[string][]string

// NewArgs returns a new Args populated with the initial args
func NewArgs() Args {
	args := Args{}
	return args
}

func (args Args) Add(key, value string) {
	vals := args[key]
	args[key] = append(vals, value)
}
