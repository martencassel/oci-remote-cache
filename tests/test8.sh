#!/bin/bash

podman rmi localhost:8080/registry.access.redhat.com/ubi9:latest||true
podman image pull localhost:8080/registry.access.redhat.com/ubi9:latest --tls-verify=false

