#!/bin/sh
set -eu
umask 077
python3 /app/scripts/runtime_config.py --root /app --runtime /app/runtime \
  --public-root /app/public --bind 0.0.0.0 --port 8080 \
  --root-url "${CYLEDGER_ROOT_URL:-http://localhost:8080/}"
if [ "$#" -gt 0 ]; then
  exec "$@"
fi
exec /app/cyledger --conf-path /app/runtime/cyledger.ini server run
