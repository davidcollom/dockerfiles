#!/bin/sh
chown -R named:named /var/cache/bind
# Run in foreground and log to STDERR (console):
/usr/sbin/named -c /etc/bind/named.conf -f -u named
