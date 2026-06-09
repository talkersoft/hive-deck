#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="$HOME/workspace/hive-deck-pro/config"
REPO_PATH="$CONFIG_DIR/workflow-configuration"
ENV_FILE="$HOME/.hv/.env"

mkdir -p "$(dirname "$ENV_FILE")"

if [ -d "$REPO_PATH" ]; then
  echo "export HV_HOME=\"$CONFIG_DIR\"" > "$ENV_FILE"
  echo "hv: wrote HV_HOME=$CONFIG_DIR to $ENV_FILE"

  for rc in ~/.bashrc ~/.zshrc; do
    if [ -f "$rc" ] && ! grep -q "hv/.env" "$rc" 2>/dev/null; then
      echo "hv: add this line to $rc to activate HV_HOME automatically:"
      echo "    [ -f ~/.hv/.env ] && source ~/.hv/.env"
    fi
  done
else
  echo "hv: workflow-configuration not found at $REPO_PATH — skipping HV_HOME"
  echo "hv: run 'hv init hive-deck-pro' to provision it first"
fi
