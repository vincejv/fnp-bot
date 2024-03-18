#!/bin/sh
set -e

if [ "${1:0:1}" = '-' ]; then
    set -- fnp-bot "$@"
fi

if [ $EUID -ne 0 ]; then
    exec "$@"
else
    # honor groups supplied via 'docker run --group-add ...' but drop 'root'
    # (also removes 'fnp-bot' since we unconditionally add it and don't want it listed twice)
    groups="fnp-bot"
    extra_groups="$(id -Gn || true)"
    for group in $extra_groups; do
        case "$group" in
            root | fnp-bot) ;;
            *) groups="$groups,$group" ;;
        esac
    done
    exec setpriv --reuid fnp-bot --regid fnp-bot --groups "$groups" "$@"
fi