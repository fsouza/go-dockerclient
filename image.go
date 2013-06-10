// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import "errors"

// Error returned when the image does not exist.
var ErrNoSuchImage = errors.New("No such image")
