#!/bin/bash -x

# Determine path to the project
fullpath="$(realpath $0)"
curdir="$(dirname ${fullpath})"
repodir="$(realpath ${curdir})"
echo "curdir=${curdir}"
echo "repodir=${repodir}"

# Docker options
dockerimg="golang:1.20.13@sha256:21089a96ccaae78cc1efbbe04ae9c8daf408ec5cedcc9872a016e8249b4bb6f7"
dockeropt="--rm -it --volume=${repodir}:/home --workdir=/home"

# Reuse existing GOPATH directory if it exists
if [[ -n "${GOPATH}" ]] && [[ -d "${GOPATH}" ]]
then
    dockeropt="${dockeropt} --volume=${GOPATH}:/go"
fi

# Run go commands in the docker container
docker run ${dockeropt} ${dockerimg} $@
