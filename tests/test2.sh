#!/bin/bash

podman image rm  localhost:8080/docker.io/alpine:latest ||true
podman image pull localhost:8080/docker.io/alpine:latest --tls-verify=false

podman image rm localhost:8080/docker.io/postgres:latest||true
podman image pull localhost:8080/docker.io/postgres:latest --tls-verify=false
