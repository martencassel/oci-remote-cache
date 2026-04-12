#!/bin/bash


podman rmi localhost:8080/gcr.io/kubeflow-images-public/alpine:latest||true
podman image pull localhost:8080/gcr.io/kubeflow-images-public/alpine:latest --tls-verify=false

