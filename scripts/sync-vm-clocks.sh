#!/usr/bin/env bash
# Configure the three UTM VMs through the existing SSH aliases.
# Cloudflare's published IPv4 NTP endpoints bypass fake-IP DNS used by proxies.
# https://developers.cloudflare.com/time-services/ntp/usage/
set -euo pipefail
if (( $# == 0 )); then
  set -- k3s-master k3s-worker-1 k3s-worker-2
fi
for task_node in "$@"; do
  ssh -o BatchMode=yes -o ConnectTimeout=5 "$task_node" 'sudo -n bash -s' <<'REMOTE'
set -euo pipefail
install -d /etc/chrony/conf.d /etc/chrony/sources.d
original=/etc/chrony/sources.d/ubuntu-ntp-pools.sources
backup=/etc/chrony/sources.d/ubuntu-ntp-pools.sources.before-utm-clock-fix
if [[ -f "$original" && ! -e "$backup" ]]; then
  mv "$original" "$backup"
fi
cat > /etc/chrony/sources.d/ride-sharing.sources <<'SOURCES'
# Official Cloudflare NTP endpoints; bypass proxy fake-IP DNS.
server 162.159.200.1 iburst minpoll 4 maxpoll 6
server 162.159.200.123 iburst minpoll 4 maxpoll 6
SOURCES
cat > /etc/chrony/conf.d/99-utm-clock.conf <<'CONFIG'
# Allow a clock step after VM suspend/resume, including after startup.
makestep 1 -1
CONFIG
chronyd -p >/dev/null
systemctl enable --now chrony >/dev/null
systemctl restart chrony
chronyc online
chronyc burst 4/4
chronyc waitsync 20 0.1 0 2
hostname
date -u '+%Y-%m-%d %H:%M:%S UTC'
timedatectl show -p NTP -p NTPSynchronized
chronyc -n sources
REMOTE
done
