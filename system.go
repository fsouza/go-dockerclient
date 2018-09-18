package docker

import (
	"context"
	"encoding/json"
	"time"
)

type VolumeUsageData struct {

	// The number of containers referencing this volume. This field
	// is set to `-1` if the reference-count is not available.
	//
	// Required: true
	RefCount int64 `json:"RefCount"`

	// Amount of disk space used by the volume (in bytes). This information
	// is only available for volumes created with the `"local"` volume
	// driver. For volumes created with other volume drivers, this field
	// is set to `-1` ("not available")
	//
	// Required: true
	Size int64 `json:"Size"`
}

type ImageSummary struct {
	Containers  int64             "json:\"Containers\""
	Created     int64             "json:\"Created\""
	ID          string            "json:\"Id\""
	Labels      map[string]string "json:\"Labels\""
	ParentID    string            "json:\"ParentId\""
	RepoDigests []string          "json:\"RepoDigests\""
	RepoTags    []string          "json:\"RepoTags\""
	SharedSize  int64             "json:\"SharedSize\""
	Size        int64             "json:\"Size\""
	VirtualSize int64             "json:\"VirtualSize\""
}

type BuildCache struct {
	ID          string
	Parent      string
	Type        string
	Description string
	InUse       bool
	Shared      bool
	Size        int64
	CreatedAt   time.Time
	LastUsedAt  *time.Time
	UsageCount  int
}

type DiskUsage struct {
	LayersSize  int64
	Images      []*ImageSummary
	Containers  []*APIContainers
	Volumes     []*Volume
	BuildCache  []*BuildCache
	BuilderSize int64 // deprecated
}

type DiskUsageOptions struct {
	Context context.Context
}

//
//
//
//
// ListImages returns the list of available images in the server.
//
// See https://goo.gl/BVzauZ for more details.
func (c *Client) DiskUsage(opts DiskUsageOptions) (*DiskUsage, error) {
	path := "/system/df"
	resp, err := c.do("GET", path, doOptions{context: opts.Context})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var du *DiskUsage
	if err := json.NewDecoder(resp.Body).Decode(&du); err != nil {
		return nil, err
	}
	return du, nil
}
