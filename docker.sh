#!/bin/bash -x

# Determine path to the project
fullpath="$(realpath $0)"
curdir="$(dirname ${fullpath})"
repodir="$(realpath ${curdir})"
echo "curdir=${curdir}"
echo "repodir=${repodir}"

# Docker options
dockerimg="golang:1.21.6@sha256:efe985ec6ca642b035ea2896c7e975f0ec04063bcdb206bbff0d4e1c74ba3ff7"
dockeropt="--rm -it --volume=${repodir}:/home --workdir=/home"

# Reuse existing GOPATH directory if it exists
if [[ -n "${GOPATH}" ]] && [[ -d "${GOPATH}" ]]
then
    dockeropt="${dockeropt} --volume=${GOPATH}:/go"
fi

# Run go commands in the docker container
docker run ${dockeropt} ${dockerimg} $@
