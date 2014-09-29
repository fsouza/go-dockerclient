// +build !go1.3

package docker

import "archive/tar"

func addXattrs(hdr *tar.Header, path string) *tar.Header {
	return hdr
}
