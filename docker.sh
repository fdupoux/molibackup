#!/bin/bash -x

# Determine path to the project
fullpath="$(realpath $0)"
curdir="$(dirname ${fullpath})"
repodir="$(realpath ${curdir})"
echo "curdir=${curdir}"
echo "repodir=${repodir}"

# Docker options
dockerimg="golang:1.22.4@sha256:969349b8121a56d51c74f4c273ab974c15b3a8ae246a5cffc1df7d28b66cf978"
dockeropt="--rm -it --volume=${repodir}:/home --workdir=/home"

# Reuse existing GOPATH directory if it exists
if [[ -n "${GOPATH}" ]] && [[ -d "${GOPATH}" ]]
then
    dockeropt="${dockeropt} --volume=${GOPATH}:/go"
fi

# Run go commands in the docker container
userid="$(id -u)"
docker run ${dockeropt} ${dockerimg} bash -c "/usr/sbin/useradd --home-dir=/home --uid ${userid} user && su -c '$*' user"
