#!/bin/bash

until [ "$(docker inspect -f {{.State.Health.Status}} notification-platform-mysql-1)" == "healthy" ]; do
  echo "等待MySQL容器就绪..."
  sleep 5
done
echo "MySQL容器已就绪!"