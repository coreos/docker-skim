#!/bin/bash

BINS="enter gc run stop"
BASEDIR=$PWD

# Verify the core binaries of go and glide exist

# Ensure the vendor directory exists
if [ ! -d vendor ]; then
    glide install

    # Remove the vendor area from github.com/coreos/rkt since this will cause
    # a cyclic dependency with run/run.go
    rm -rf vendor/github.com/coreos/rkt/vendor
fi

# Build the actool
pushd vendor/github.com/appc/spec/actool
go build
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
    pushd $i && go build
    cp $i ../target/rootfs
    popd
done

# Generate the aci image
cp aci/aci-manifest.in target/manifest
aci/actool build target stage1-skim.aci
