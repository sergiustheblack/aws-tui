#!/bin/bash
# Drives ./aws-tui in a detached tmux session against hack/fake_aws.py, with
# an isolated environment (no real AWS credentials or config are used).
#
#   hack/tui.sh start [extra env...]  start fake endpoint + app, print screen
#   hack/tui.sh keys <keys...>        tmux send-keys, then print screen
#   hack/tui.sh show                  print screen
#   hack/tui.sh stop                  stop app and fake endpoint
#
# Example: hack/tui.sh start AWS_PROFILE=p1; hack/tui.sh keys / d b
set -euo pipefail
cd "$(dirname "$0")/.."
PORT=18555
tm() { tmux -L aws-tui "$@"; }

case "${1:-}" in
start)
  shift
  tm kill-server 2>/dev/null || true
  pgrep -f "hack/fake_aws.py $PORT" >/dev/null || (python3 -I hack/fake_aws.py $PORT &>/dev/null &)
  tm new-session -d -s t -x 160 -y 40 \
    "env -i HOME=/nonexistent PATH=/usr/bin:/bin TERM=xterm-256color \
     AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null \
     AWS_ACCESS_KEY_ID=AKIDTEST AWS_SECRET_ACCESS_KEY=secret \
     AWS_EC2_METADATA_DISABLED=true AWS_ENDPOINT_URL=http://127.0.0.1:$PORT \
     $* ./aws-tui; echo EXITED \$?; sleep 600"
  sleep 2; tm capture-pane -p -t t ;;
keys) shift; tm send-keys -t t "$@"; sleep 1; tm capture-pane -p -t t ;;
show) tm capture-pane -p -t t ;;
stop) tm kill-server 2>/dev/null || true; pkill -f "hack/fake_aws.py $PORT" || true ;;
*) sed -n 2,10p "$0"; exit 1 ;;
esac
