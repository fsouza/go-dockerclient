// Copyright 2014 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package docker

/*
ByCreatedDescending is a type that wraps []APIImages so that it can be
sorted using the "sort" package Interface.  It sorts by the created value in
descending order, which means the newest image will appear first
*/
type ByCreatedDescending []APIImages

func (slice ByCreatedDescending) Len() int {
	return len(slice)
}

func (slice ByCreatedDescending) Less(i, j int) bool {
	return slice[i].Created > slice[j].Created
}

func (slice ByCreatedDescending) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

/*
ByCreatedAscending is a type that wraps []APIImages so that it can be
sorted using the "sort" package Interface.  It sorts by the created value in
descending order, which means the oldest image will appear first
*/
type ByCreatedAscending []APIImages

func (slice ByCreatedAscending) Len() int {
	return len(slice)
}

func (slice ByCreatedAscending) Less(i, j int) bool {
	return slice[i].Created < slice[j].Created
}

func (slice ByCreatedAscending) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}
