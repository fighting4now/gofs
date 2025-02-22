#!/usr/bin/env bash

# update git repository
# git pull --no-rebase

# update the latest golang image
# docker pull golang

# set the gofs docker image name by GOFSIMAGENAME environment variable
export GOFSIMAGENAME=nosrc/gofs

# set the gofs docker image tag by GOFSIMAGETAG environment variable
export GOFSIMAGETAG=latest

# remove the existing old image
docker rmi -f $GOFSIMAGENAME:$GOFSIMAGETAG

# build Dockerfile
docker build --build-arg GOPROXY=$GOPROXY -t $GOFSIMAGENAME:$GOFSIMAGETAG .

# remove dangling images
docker rmi -f $(docker images -q -f "dangling=true")

# run a container to print the gofs version
docker run -it --rm --name running-gofs-version $GOFSIMAGENAME:$GOFSIMAGETAG gofs -v
