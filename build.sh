#!/bin/bash

GO=/usr/local/go/bin/go
if [ -f "$GO" ]; then
    /usr/local/go/bin/go get
    /usr/local/go/bin/go mod vendor
else 
    go get
    go mod vendor
fi

TIMESTAMP=$(date +"%Y%m%d%H%M%S")
docker build -f Dockerfile -t 3dsinteractive/pam-demo-server:$TIMESTAMP .
docker push 3dsinteractive/pam-demo-server:$TIMESTAMP