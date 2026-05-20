---
name: mcoa-task-creator
description: >-
  Convert an approved MCOA impact map into structured development tasks.
  Use after impact map review for multicluster-observability-addon.
---
# MCOA Structured Task Creator

Convert approved MCOA impact maps into implementation-ready tasks.

## MCOA-Specific Rules

1. **Layer separation** — keep controller, addon-core, signal-package, and Helm chart changes in logical tasks
2. **Signal tagging** — mark each task with the signal type (metrics/logs/traces/coo/analytics)
3. **Helm chart awareness** — if a signal package changes, there's usually a corresponding chart change
4. **AddOnDeploymentConfig** — if new config keys are needed, create a separate task for docs/README

## Workflow

1. Read the approved impact map from `docs/impact-maps/`
2. Read `templates/task-template.md`
3. Verify file paths still exist
4. Find Go symbols referenced in the impact map
5. Order: shared addon-core changes → signal packages → Helm charts → tests
6. Save to `docs/tasks/<feature-slug>/task-<N>.md`
