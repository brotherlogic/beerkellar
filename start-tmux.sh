#!/bin/bash

# Ensure the 'prod' session exists
if ! tmux has-session -t beerkellar 2>/dev/null; then
  # Create a new session named 'prod', detached
  cd /workspaces/beerkellar
  tmux new-session -d -s beerkellar
fi
