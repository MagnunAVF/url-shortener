#!/bin/sh
set -eu

mkdir -p /var/log/app /var/lib/vector /etc/vector
# Start Vector in background if config exists
if [ -f /etc/vector/vector.toml ]; then
  # Validate config to fail fast
  vector validate --config-toml /etc/vector/vector.toml || exit 1
  vector --config /etc/vector/vector.toml &
fi

# Start the main Go service
exec /root/main
