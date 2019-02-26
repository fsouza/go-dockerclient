#!/bin/bash -ex

# Copyright 2016 go-dockerclient authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

if [[ $TRAVIS_OS_NAME == "windows" ]]; then
	export SKIP_FMT_CHECK=1
fi

make staticcheck fmtcheck gotest

if [[ $TRAVIS_OS_NAME == "linux" || $TRAVIS_OS_NAME == "windows" ]]; then
	make integration
fi
