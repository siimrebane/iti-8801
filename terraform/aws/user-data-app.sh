#!/bin/bash
set -e
apt-get update
apt-get install -y docker.io
systemctl enable --now docker
docker run -d --name beacon --restart=always --network host -e DB_TARGET=${db_target} ${beacon_image}
