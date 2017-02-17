#!/bin/bash

# Copyright 2017 CoreOS
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

ROOT=$(dirname "${BASH_SOURCE}")
source "${ROOT}/scripts/util.sh"

BINS="enter gc run stop"

eval $(go env)
export GOBIN=${PWD}/bin/${GOARCH}
export CGO_ENABLED=1

# Clean the repo, but save the vendor area
if [ "x$1" != "x" ] && [ "clean" == "$1" ]; then
    echo "cleaning project"
    rm -rf target
    rm -f stage1-skim.aci
    rm -f aci/actool
    rm -f "${GP_DIR}"
    rmdir --parents "gopath/src/$ORG_PATH" || true

    for i in $BINS; do
        rm -f $i/$i
    done

    exit 0
fi

util::symlink_gopath

# Build actool
pushd "$GP_DIR"
trap 'popd' EXIT

go build -o ./aci/actool "${REPO_PATH}/vendor/github.com/appc/spec/actool"

# Build up the target directory and the rootfs
if [ ! -d target ]; then
    mkdir -p target/rootfs
    mkdir -p target/rootfs/opt/stage2
    mkdir -p target/rootfs/rkt/status
    cd target/rootfs && ln -s flavor skim && cd ../..
fi

for i in $BINS; do
    go build -o ./$i/$i ./$i/
    cp $i/$i target/rootfs
done

# Generate the aci image
cp aci/aci-manifest.in target/manifest

if [ -f stage1-skim.aci ]; then
    rm stage1-skim.aci
fi

./aci/actool build target stage1-skim.aci
