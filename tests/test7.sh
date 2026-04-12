#!/bin/bash

podman rmi localhost:8080/mcr.microsoft.com/aks/periscope:v0.6||true
podman image pull localhost:8080/mcr.microsoft.com/aks/periscope:v0.6 --tls-verify=false

