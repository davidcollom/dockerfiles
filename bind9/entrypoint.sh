#!/bin/sh

echo "Setting Permissions..."
chown -R named:named /var/cache/bind

echo "Starting Named..."
# Run in foreground and log to STDERR (console):
exec /usr/sbin/named -c /etc/bind/named.conf -f -u named
