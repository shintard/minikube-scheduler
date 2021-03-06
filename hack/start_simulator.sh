#!/usr/bin/env bash

source "./hack/etcd.sh"

checkEtcdOnPath() {
  echo "Checking etcd is on PATH"
  which etcd && return
  echo "Cannot find etcd on PATH."
  exit 1
}

CLEANUP_REQUIRED=
start_etcd() {
  echo "Starting etcd instance"
  CLEANUP_REQUIRED=1
  kube::etcd::start
  echo "etcd started"
}

cleanup_etcd() {
  if [[ -z "${CLEANUP_REQUIRED}" ]]; then
    return
  fi
  echo "Cleaning up etcd"
  kube::etcd::cleanup
  CLEANUP_REQUIRED=
  echo "Clean up finished"
}

checkEtcdOnPath

trap cleanup_etcd EXIT

start_etcd

PORT=1212 FRONTEND_URL=http://localhost:3000 ./bin/main


