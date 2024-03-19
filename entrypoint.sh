#!/bin/sh
set -e

if [ "${1:0:1}" = '-' ]; then
    set -- fnp-bot "$@"
fi

if [ "$(id -u)" -ne 0 ]; then
    exec "$@"
else
    exec su-exec fnp-bot "$@"
fi