#!/usr/bin/env bash

# Determine path to the project
fullpath="$(realpath $0)"
curdir="$(dirname ${fullpath})"
repodir="$(realpath ${curdir})"
echo "curdir=${curdir}"
echo "repodir=${repodir}"

# Docker options
dockerimg="golang:1.25.5@sha256:36b4f45d2874905b9e8573b783292629bcb346d0a70d8d7150b6df545234818f"
dockeropt="--rm -it --volume=${repodir}:/home --workdir=/home"

# Reuse existing GOPATH directory if it exists
if [[ -n "${GOPATH}" ]] && [[ -d "${GOPATH}" ]]
then
    dockeropt="${dockeropt} --volume=${GOPATH}:/go"
fi

# Run go commands in the docker container
userid="$(id -u)"
docker run ${dockeropt} ${dockerimg} bash -c "/usr/sbin/useradd --home-dir=/home --uid ${userid} user && su -c '$*' user"
