#!/bin/bash

podman rmi localhost:8080/jfrog-docker-reg2.bintray.io/jfrog/artifactory-pro:latest||true
podman image pull localhost:8080/jfrog-docker-reg2.bintray.io/jfrog/artifactory-pro:latest --tls-verify=false
