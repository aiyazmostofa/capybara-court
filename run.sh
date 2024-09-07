#!/bin/bash
#docker build -t capybara-court .
docker run --rm --stop-timeout 0 -m 512m --cpus=0.5 --net none --ulimit nproc=128 -v $2:/app:ro -i capybara-court ../jdk/bin/java $1
