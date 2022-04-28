#!/usr/bin/env bash

mkdir -p remote-push-client/source remote-push-client/dest

# depending on your situation, set the gofs server address
export GOFS_SERVER_ADDR=10.0.4.8

docker run -it --rm -v "$PWD":/workspace --name running-gofs-remote-push-client nosrc/gofs:latest \
  gofs -source="./remote-push-client/source" -dest="rs://$GOFS_SERVER_ADDR:8105?local_sync_disabled=false&path=./remote-push-client/dest" -users="gofs|password"
