// +build go1.3

package docker

import (
	"archive/tar"

	"github.com/docker/docker/pkg/system"
)

func addXattrs(hdr *tar.Header, path string) *tar.Header {
	capability, _ := system.Lgetxattr(path, "security.capability")
	if capability != nil {
		hdr.Xattrs = make(map[string]string)
		hdr.Xattrs["security.capability"] = string(capability)
	}

	return hdr
}
