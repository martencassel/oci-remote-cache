#!/bin/bash

podman rmi public.ecr.aws/amazonlinux/amazonlinux:2.0.20250623.0||true
podman image pull localhost:8080/public.ecr.aws/amazonlinux/amazonlinux:2.0.20250623.0 --tls-verify=false

