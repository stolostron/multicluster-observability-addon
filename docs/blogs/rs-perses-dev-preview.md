# Right-Sizing Recommendations with MCOA and Perses Dashboards: A Developer Preview

**Authors:** [Darshan Vandra](https://developers.redhat.com/author/darshan-vandra), [Raj Zalavadia](https://developers.redhat.com/author/raj-zalavadia)

**Related products:** [Red Hat Advanced Cluster Management for Kubernetes](https://www.redhat.com/en/technologies/management/advanced-cluster-management), [Red Hat OpenShift Virtualization](https://www.redhat.com/en/technologies/cloud-computing/openshift/virtualization)

---

## Table of contents

- [Introduction](#introduction)
- [Why Perses?](#why-perses)
- [Namespace right-sizing with Perses](#namespace-right-sizing-with-perses)
- [OpenShift Virtualization right-sizing with Perses](#openshift-virtualization-right-sizing-with-perses)
- [Getting started](#getting-started)
- [What's next](#whats-next)

---

## Introduction

Since the [general availability of right-sizing recommendations in Red Hat Advanced Cluster Management 2.16](https://developers.redhat.com/articles/2026/03/17/advanced-cluster-management-216-right-sizing-recommendation-ga), Right-sizing is available for platform engineers and FinOps teams to leverage Grafana-based dashboards to identify over-provisioned and under-utilized resources across their multicluster environments. The feature has matured through [developer preview](https://developers.redhat.com/articles/2024/07/16/improved-right-sizing-experience-red-hat-advanced-cluster-management-kubernetes), [technology preview for namespaces](https://developers.redhat.com/articles/2025/08/04/optimize-workloads-right-sizing-recommendations) and [OpenShift Virtualization](https://developers.redhat.com/articles/2025/12/05/right-sizing-recommendations-openshift-virtualization), and ultimately reached GA.

Today, we are excited to announce the **developer preview** of right-sizing recommendations powered by [Perses](https://perses.dev/) dashboards, delivered through the **Multicluster Observability Addon (MCOA)** as part of the **Red Hat Advanced Cluster Management 2.17** release. This represents a process of significant architectural evolution—migrating from Grafana to Perses as the visualization layer while also moving the right-sizing logic into the MCOA for a cleaner, more modular deployment model.

This developer preview covers:

- **Namespace-level right-sizing** with Perses dashboards
- **OpenShift Virtualization right-sizing** with Perses dashboards

## Why Perses?

[Perses](https://perses.dev/) is a CNCF project that provides a modern, open-source dashboard solution designed specifically for cloud-native observability. The migration from Grafana to Perses brings several advantages:

1. **Native Kubernetes integration:** Perses dashboards are defined as code using the Perses Go SDK, enabling version-controlled, programmatically generated dashboards that live alongside the application code.

2. **Tighter ACM console integration:** Perses dashboards are embedded directly into the Red Hat Advanced Cluster Management console monitoring experience, providing seamless navigation between cluster management and resource optimization views.

3. **Interactive data links:** Tables support clickable links that drill down from namespace-level overview to workload-level and pod-level detail views, enabling a fluid top-down investigation flow.

4. **Open-source sustainability:** As a CNCF project, Perses aligns with Red Hat's commitment to open-source ecosystems and community-driven innovation.

5. **Dashboards as code:** Right-sizing dashboards are defined in Go using the Perses SDK, making them testable, reviewable, and maintainable through standard software engineering practices.

## Namespace right-sizing with Perses

The **ACM Right-Sizing Namespace** Perses dashboard provides a comprehensive view of resource allocation efficiency across all namespaces in a selected cluster.

Start by selecting the cluster you would like to explore, the recommendation profile, and the preferred aggregation period from the dropdown variables at the top of the dashboard. The dashboard provides:

<!-- Figure: ACM Right-Sizing Namespace Perses dashboard screenshot -->

**Cluster-level summary:** Four stat panels at the top show CPU Recommendation, CPU Usage, CPU Request, and CPU Utilization for the selected cluster. The same four metrics are presented for memory, giving administrators a complete picture of cluster resource efficiency. Utilization is color-coded—green for efficient, red for underutilized, yellow for over-utilized.

**Top namespaces utilization:** A time-series chart displays the top 20 namespaces by CPU and memory utilization ratio over time, enabling quick identification of the most over-provisioned or under-provisioned namespaces.

**Namespace quota tables:** Detailed tables for both CPU and memory display per-namespace Utilization %, Usage, Request, Recommendation, and Request Hard.

<!-- Figure: Namespace right-sizing Perses dashboard screenshot -->

## OpenShift Virtualization right-sizing with Perses

The **ACM Right-Sizing OpenShift Virtualization** Perses dashboard provides purpose-built views for virtual machine resource optimization.

<!-- Figure: VM right-sizing overview Perses dashboard screenshot -->

**VM overview:** Four stat panels summarize the current state across all VMs in the selected cluster and namespace—Total CPU Overestimation (cores that can be reclaimed, highlighted in red), Total CPU Underestimation (additional cores needed, highlighted in yellow), Total Memory Overestimation, and Total Memory Underestimation. These aggregate views only consider running VMs, ensuring recommendations are based on active workloads.

**Per-VM tables:** Four interactive tables provide granular per-VM analysis for CPU Overestimation, CPU Underestimation, Memory Overestimation, and Memory Underestimation. Each table shows VM Name, Namespace, Utilization %, Usage, Request, Recommendation, and the overestimation/underestimation delta.

Each VM name is a clickable link that navigates to a dedicated detail dashboard showing:

- Stat panels for CPU/Memory overestimation or underestimation, usage, request, and utilization
- Time-series charts depicting CPU and memory utilization over time for the selected VM

The information can be filtered by cluster, namespace, and preferred time aggregation (e.g., 5/10/30/60/90 days).

<!-- Figure: VM detail Perses dashboard screenshot -->

## Getting started

### Prerequisites

- Red Hat Advanced Cluster Management for Kubernetes 2.17 <!-- [official doc link] -->
- Multicluster Observability Operator (MCO) installed on the hub
- Multicluster Observability Addon (MCOA) deployed
- Observability components (Prometheus, Thanos, Perses) active
- For virtualization right-sizing: OpenShift Virtualization enabled on managed clusters

### Enabling MCOA from MCO

The Multicluster Observability Addon (MCOA) is automatically deployed by MCO when any capability is enabled in the `MultiClusterObservability` CR's `capabilities` field. To enable MCOA with right-sizing, configure the `capabilities` section in your `MultiClusterObservability` CR:

```yaml
apiVersion: observability.open-cluster-management.io/v1beta2
kind: MultiClusterObservability
metadata:
  name: observability
spec:
  capabilities:
    platform:
      analytics:
        namespaceRightSizingRecommendation:
          enabled: true
        virtualizationRightSizingRecommendation:
          enabled: true
  observabilityAddonSpec: {}
  storageConfig:
    metricObjectStorage:
      name: thanos-object-storage
      key: thanos.yaml
```

Once the CR is applied, MCO will deploy MCOA on the hub cluster. You can verify the addon is installed:

```shell
kubectl get ClusterManagementAddOn multicluster-observability-addon
```

MCOA will then automatically install on spoke clusters and manage the right-sizing resources.

### Right-sizing with metrics collection

Right-sizing can operate in two modes depending on whether platform metrics collection is enabled:

**With metrics collection enabled:**

When platform metrics collection is enabled alongside right-sizing, MCOA manages both the metrics pipeline (via PrometheusAgent) and the right-sizing recording rules on managed clusters. This is the recommended configuration for full observability coverage.

```yaml
spec:
  capabilities:
    platform:
      metrics:
        default:
          enabled: true
      analytics:
        namespaceRightSizingRecommendation:
          enabled: true
        virtualizationRightSizingRecommendation:
          enabled: true
```

**Without metrics collection (right-sizing standalone):**

Right-sizing can also function independently without enabling platform metrics collection. In this mode, MCOA deploys only the right-sizing Prometheus recording rules to managed clusters via ManifestWorks. The recording rules compute the `acm_rs:*` metrics locally, which are then forwarded to the hub through the existing observability pipeline.

```yaml
spec:
  capabilities:
    platform:
      analytics:
        namespaceRightSizingRecommendation:
          enabled: true
        virtualizationRightSizingRecommendation:
          enabled: true
```

In both modes, right-sizing will automatically:

1. Deploy Prometheus recording rules to managed clusters via ManifestWorks
2. Configure metric forwarding through the observability pipeline
3. Register Perses dashboards with the hub's Perses server

### Accessing the dashboards

Once enabled, access the dashboards through the ACM console under **Observability** → **Dashboards**, or navigate directly to **ACM Right-Sizing Namespace** and **ACM Right-Sizing OpenShift Virtualization**.

### Disclaimers

As this is a developer preview: The Perses dashboards are under active development and may change in future releases.

## What's next

This developer preview marks the beginning of the transition from Grafana to Perses for right-sizing visualization in Red Hat Advanced Cluster Management. We are working on the technology preview release with expanded capabilities including workload and pod-level right-sizing.

We value your feedback, which is crucial for shaping the future of right-sizing in Red Hat Advanced Cluster Management. Share your questions and recommendations with us using the [Red Hat OpenShift feedback form](https://redhatdg.co1.qualtrics.com/jfe/form/SV_6X9h8MnPno3eg86?source=observability).
