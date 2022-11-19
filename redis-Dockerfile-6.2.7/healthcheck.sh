#!/bin/bash

check_redis_health() {
  if [[ -z "${REDIS_PASSWORD}" ]]; then
    redis-cli ping
  else
    # redis-cli --user default --pass 123 ping
    redis-cli --user "default" --pass "${REDIS_PASSWORD}" ping
  fi
}

check_redis_health
