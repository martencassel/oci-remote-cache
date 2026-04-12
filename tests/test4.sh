#!/bin/bash

podman rmi localhost:8080/ghcr.io/windsource/nextcloud-influxdb-tracks-importer:1.0.0||true
podman image pull localhost:8080/ghcr.io/windsource/nextcloud-influxdb-tracks-importer:1.0.0 --tls-verify=false

