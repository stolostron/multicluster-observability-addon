#!/bin/bash
input=$(cat)

if [ -d "docs/impact-maps" ]; then
  latest_map=$(ls -t docs/impact-maps/*.md 2>/dev/null | head -1)
  if [ -n "$latest_map" ]; then
    if ! grep -q "APPROVED" "$latest_map" 2>/dev/null; then
      echo '{
        "followup_message": "An MCOA impact map was created but not yet approved. Ask the human to review docs/impact-maps/ before proceeding. Check: were all signal types and layers analyzed?"
      }'
      exit 0
    fi
  fi
fi

echo '{}'
exit 0
