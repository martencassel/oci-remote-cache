#!/bin/bash

podman rmi localhost:8080/quay.io/argoproj/argocd:v2.7.3||true
podman image pull localhost:8080/quay.io/argoproj/argocd:v2.7.3 --tls-verify=false

