#!/bin/bash

docker rm -f $(docker ps -aq)

# let's get a few basic containers going
docker run -d --name redis1 redis
docker run -d --name redis2 redis
docker run -d --name data-container -v /data busybox true

# now let's really let 'em have it
for i in $(seq 0 $1); do
    docker run -d --link redis1:redis1 redis 
    docker run -d --link redis1:redis1 --link redis2:redis2 redis 
    docker run -d --link redis2:redis2 redis 
    docker run -d --volumes-from data-container busybox true  
    docker run -d --volumes-from data-container busybox sh -c 'while true; do echo hello world; sleep 1; done' 
done
