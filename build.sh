#!/bin/bash

# Copyright 2017 The rkt Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -e

BINS="enter gc run stop"
BASEDIR=$PWD

# Verify the core binaries of go and glide exist
GLIDEBIN=`which glide`
GOBIN=`which go`

if [ "x$GLIDEBIN" == "x" ]; then
    echo "Glide needs to be in your path"
    exit 254
fi

if [ "x$GOBIN" == "x" ]; then
    echo "The go binary needs to be in your path"
    exit 254
fi

# Clean the repo, but save the vendor area
if [ "x$1" != "x" ] && [ "clean" == $1 ]; then
    echo "cleaning project"
    rm -rf target
    rm -f stage1-skim.aci
    rm -f aci/actool

    for i in $BINS; do
        rm -f $i/$i
    done

    exit 0
fi

# Ensure the vendor directory exists
if [ ! -d vendor ]; then
    $GLIDEBIN install

    # Remove the vendor area from github.com/coreos/rkt since this will cause
    # a cyclic dependency with run/run.go
    rm -rf vendor/github.com/coreos/rkt/vendor
fi

# Build the actool
pushd vendor/github.com/appc/spec/actool
$GOBIN build
mv actool $BASEDIR/aci
popd

# Build up the target directory and the rootfs
if [ ! -d target ]; then
    mkdir -p target/rootfs
    mkdir -p target/rootfs/opt/stage2
    mkdir -p target/rootfs/rkt/status
    cd target/rootfs && ln -s flavor skim && cd ../..
fi

for i in $BINS; do
    pushd $i && $GOBIN build
    cp $i ../target/rootfs
    popd
done

# Generate the aci image
cp aci/aci-manifest.in target/manifest

if [ -f stage1-skim.aci ]; then
    rm stage1-skim.aci
fi

aci/actool build target stage1-skim.aci
