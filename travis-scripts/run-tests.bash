#!/bin/bash -ex

# Copyright 2016 go-dockerclient authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

if ! [[ $TRAVIS_GO_VERSION =~ ^1\.[34] ]]; then
	make lint
fi

make vet fmtcheck gotest
