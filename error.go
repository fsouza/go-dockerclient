package docker

// Abstract-like base struct for assertable errors
type DockerClientError struct {
	Message string
}

// Error() implements the error interface
func (d *DockerClientError) Error() string {
	return d.Message
}
