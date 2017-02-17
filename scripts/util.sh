#!/bin/bash

ROOT=$(dirname "${BASH_SOURCE}")/..

NAME="docker-skim"
ORG_PATH="github.com/coreos"
REPO_PATH="${ORG_PATH}/${NAME}"

# The path to this package within the symlink-created GOPATH
GP_DIR="${ROOT}/gopath/src/${REPO_PATH}"

function util::symlink_gopath {
  if [[ ! -L "${GP_DIR}" ]]; then
      mkdir -p gopath/src/${ORG_PATH}
      ln -s ../../../.. "${GP_DIR}" || exit 255
  fi

  export GOPATH="$(readlink -f "${ROOT}/gopath")"
}
