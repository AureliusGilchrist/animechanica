# 1) If a systemd service exists, stop it (system + user scope)
if command -v systemctl >/dev/null 2>&1; then
  sudo systemctl stop seanime 2>/dev/null || true
  systemctl --user stop seanime 2>/dev/null || true
fi

# 2) Show who is listening on :43211
echo "Before:"
if command -v lsof >/dev/null 2>&1; then
  lsof -nP -i TCP:43211 -sTCP:LISTEN || true
else
  ss -ltnp "sport = :43211" || true
fi

# 3) Gracefully terminate any listeners on :43211
if command -v lsof >/dev/null 2>&1; then
  PIDS=$(lsof -nP -t -i TCP:43211 -sTCP:LISTEN | sort -u || true)
else
  PIDS=$(ss -ltnp "sport = :43211" 2>/dev/null | awk 'NR>1 {print $NF}' | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p' | sort -u || true)
fi

if [ -n "$PIDS" ]; then
  echo "Sending SIGTERM to: $PIDS"
  for PID in $PIDS; do kill -TERM "$PID" 2>/dev/null || true; done
  sleep 2
fi

# 4) Force kill if still listening
if command -v lsof >/dev/null 2>&1; then
  REMAIN=$(lsof -nP -t -i TCP:43211 -sTCP:LISTEN | sort -u || true)
else
  REMAIN=$(ss -ltnp "sport = :43211" 2>/dev/null | awk 'NR>1 {print $NF}' | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p' | sort -u || true)
fi
if [ -n "$REMAIN" ]; then
  echo "Force killing: $REMAIN"
  for PID in $REMAIN; do kill -KILL "$PID" 2>/dev/null || true; done
fi

# 5) Verify port is free
echo "After:"
if command -v lsof >/dev/null 2>&1; then
  lsof -nP -i TCP:43211 -sTCP:LISTEN || echo "Port 43211 is now free."
else
  ss -ltnp "sport = :43211" || true
fi
