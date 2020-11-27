package docker

import "encoding/json"

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

func (args Args) Len() int {
	return len(args)
}

// ToJSON returns the Args as a JSON encoded string
func (args Args) ToJSON() string {
	if args.Len() == 0 {
		return ""
	}
	buf, _ := json.Marshal(args)
	return string(buf)
}