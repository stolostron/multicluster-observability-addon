#!/bin/bash
input=$(cat)
file=$(echo "$input" | jq -r '.filePath // empty')

if [ -z "$file" ]; then
  echo '{}'
  exit 0
fi

context=""

# Signal package changed — remind about Helm chart
if [[ "$file" == internal/logging/* ]] || [[ "$file" == internal/tracing/* ]] || [[ "$file" == internal/metrics/* ]] || [[ "$file" == internal/coo/* ]]; then
  signal=$(echo "$file" | sed 's|internal/\([^/]*\)/.*|\1|')
  if [[ "$file" != *"_test.go" ]]; then
    context="Signal package '$signal' was modified. Check if the corresponding Helm chart in internal/addon/manifests/charts/mcoa/charts/$signal/ also needs updating."
  fi
fi

# main.go changed — remind about scheme registration
if [[ "$file" == "main.go" ]]; then
  context="main.go was modified. If adding new API types, verify the scheme registration is complete."
fi

if [ -n "$context" ]; then
  echo "{\"additional_context\": \"$context\"}"
else
  echo '{}'
fi
exit 0
