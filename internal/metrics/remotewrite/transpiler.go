package remotewrite

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	cooprometheusv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	cooprometheusv1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"k8s.io/utils/ptr"
)

func Transpile(scrapeConfig *cooprometheusv1alpha1.ScrapeConfig, agent *cooprometheusv1alpha1.PrometheusAgent) ([]*cooprometheusv1.RemoteWriteSpec, error) {
	if scrapeConfig == nil {
		return nil, nil
	}

	matchersList, ok := scrapeConfig.Spec.Params["match[]"]
	if !ok || len(matchersList) == 0 {
		return nil, nil
	}

	parsedSelectors := make([][]*labels.Matcher, 0, len(matchersList))
	for _, mStr := range matchersList {
		matchers, err := parser.ParseMetricSelector(mStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metric selector %q: %w", mStr, err)
		}
		if len(matchers) > 0 {
			parsedSelectors = append(parsedSelectors, matchers)
		}
	}

	if len(parsedSelectors) == 0 {
		return nil, nil
	}

	estimatedRelabels := len(parsedSelectors)*2 + 4 + len(scrapeConfig.Spec.MetricRelabelConfigs)
	relabelConfigs := make([]cooprometheusv1.RelabelConfig, 0, estimatedRelabels)

	// 1. Process each selector individually to handle negation (OR disjunction semantics)
	for i, sel := range parsedSelectors {
		type posMatcher struct {
			name  string
			value string
		}

		posMatchers := make([]posMatcher, 0, len(sel))
		for _, lm := range sel {
			if lm.Type == labels.MatchEqual || lm.Type == labels.MatchRegexp {
				var val string
				if lm.Type == labels.MatchEqual {
					val = regexp.QuoteMeta(lm.Value)
				} else {
					val = "(?:" + lm.Value + ")"
				}
				posMatchers = append(posMatchers, posMatcher{
					name:  lm.Name,
					value: val,
				})
			}
		}

		// Deterministically sort by label name, sub-sorting by regex value for stability
		slices.SortFunc(posMatchers, func(a, b posMatcher) int {
			if c := cmp.Compare(a.name, b.name); c != 0 {
				return c
			}
			return cmp.Compare(a.value, b.value)
		})

		sourceLabels := make([]cooprometheusv1.LabelName, len(posMatchers))
		posValues := make([]string, len(posMatchers))
		for j, pm := range posMatchers {
			sourceLabels[j] = cooprometheusv1.LabelName(pm.name)
			posValues[j] = pm.value
		}

		// Positive Matchers Phase (Initialize __tmp_keep_i to "keep" if metric matches positive selectors)
		tmpKeepLabel := "__tmp_keep_" + strconv.Itoa(i)
		if len(sourceLabels) > 0 {
			relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
				Action:       "replace",
				SourceLabels: sourceLabels,
				Regex:        strings.Join(posValues, ";"),
				TargetLabel:  tmpKeepLabel,
				Replacement:  ptr.To("keep"),
			})
		} else {
			relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
				Action:      "replace",
				TargetLabel: tmpKeepLabel,
				Replacement: ptr.To("keep"),
			})
		}

		// Negative Matchers Phase (Clear __tmp_keep_i to "" if any negative matcher matches)
		for _, lm := range sel {
			if lm.Type == labels.MatchNotEqual || lm.Type == labels.MatchNotRegexp {
				var regexVal string
				if lm.Type == labels.MatchNotEqual {
					regexVal = regexp.QuoteMeta(lm.Value)
				} else {
					regexVal = "(?:" + lm.Value + ")"
				}

				relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
					Action:       "replace",
					SourceLabels: []cooprometheusv1.LabelName{cooprometheusv1.LabelName(tmpKeepLabel), cooprometheusv1.LabelName(lm.Name)},
					Regex:        "keep;" + regexVal,
					TargetLabel:  tmpKeepLabel,
					Replacement:  ptr.To(""),
				})
			}
		}
	}

	// 2. Initialize global __tmp_keep to "drop"
	relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
		Action:      "replace",
		TargetLabel: "__tmp_keep",
		Replacement: ptr.To("drop"),
	})

	// 3. Combine selector decisions: set global __tmp_keep to "keep" if any __tmp_keep_i is "keep" (OR logic)
	combineSourceLabels := make([]cooprometheusv1.LabelName, len(parsedSelectors))
	for i := range parsedSelectors {
		combineSourceLabels[i] = cooprometheusv1.LabelName("__tmp_keep_" + strconv.Itoa(i))
	}

	relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
		Action:       "replace",
		SourceLabels: combineSourceLabels,
		Regex:        ".*keep.*",
		TargetLabel:  "__tmp_keep",
		Replacement:  ptr.To("keep"),
	})

	// 4. Keep only metrics flagged with "keep"
	relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
		Action:       "keep",
		SourceLabels: []cooprometheusv1.LabelName{"__tmp_keep"},
		Regex:        "keep",
	})

	// 5. Cleanup all temporary labels
	relabelConfigs = append(relabelConfigs, cooprometheusv1.RelabelConfig{
		Action: "labeldrop",
		Regex:  "__tmp_keep.*",
	})

	// 6. Append custom metricRelabelings from scrapeConfig directly (safely deep-copied)
	for _, cfg := range scrapeConfig.Spec.MetricRelabelConfigs {
		relabelConfigs = append(relabelConfigs, *cfg.DeepCopy())
	}

	if agent == nil || len(agent.Spec.RemoteWrite) == 0 {
		baseSpec := &cooprometheusv1.RemoteWriteSpec{
			WriteRelabelConfigs: relabelConfigs,
		}
		return []*cooprometheusv1.RemoteWriteSpec{baseSpec}, nil
	}

	specs := make([]*cooprometheusv1.RemoteWriteSpec, 0, len(agent.Spec.RemoteWrite))
	for _, agentRw := range agent.Spec.RemoteWrite {
		relabelConfigsCopy := make([]cooprometheusv1.RelabelConfig, len(relabelConfigs), len(relabelConfigs)+len(agentRw.WriteRelabelConfigs))
		for i, cfg := range relabelConfigs {
			cfg.DeepCopyInto(&relabelConfigsCopy[i])
		}

		// Append identification relabel configs from the original agent remoteWrite
		// (e.g. cluster/clusterID label assignments) after our filtering rules.
		// Federation-specific relabel rules (exported_job/exported_instance) from
		// PrometheusAgent are skipped because native in-cluster Prometheus already
		// retains its original job and instance labels.
		for _, cfg := range agentRw.WriteRelabelConfigs {
			if isFederationRelabelConfig(cfg) {
				continue
			}
			relabelConfigsCopy = append(relabelConfigsCopy, *cfg.DeepCopy())
		}

		spec := &cooprometheusv1.RemoteWriteSpec{
			WriteRelabelConfigs: relabelConfigsCopy,
		}

		spec.URL = agentRw.URL

		if agentRw.RemoteTimeout != nil {
			spec.RemoteTimeout = ptr.To(*agentRw.RemoteTimeout)
		}
		if agentRw.BasicAuth != nil {
			spec.BasicAuth = agentRw.BasicAuth.DeepCopy()
		}
		if agentRw.Authorization != nil {
			spec.Authorization = agentRw.Authorization.DeepCopy()
		}
		if agentRw.OAuth2 != nil {
			spec.OAuth2 = agentRw.OAuth2.DeepCopy()
		}
		if agentRw.QueueConfig != nil {
			spec.QueueConfig = agentRw.QueueConfig.DeepCopy()
		}
		if agentRw.TLSConfig != nil {
			spec.TLSConfig = agentRw.TLSConfig.DeepCopy()
		}
		if agentRw.ProxyURL != nil {
			spec.ProxyURL = ptr.To(*agentRw.ProxyURL)
		}
		if agentRw.NoProxy != nil {
			spec.NoProxy = ptr.To(*agentRw.NoProxy)
		}
		if agentRw.Headers != nil {
			spec.Headers = make(map[string]string)
			maps.Copy(spec.Headers, agentRw.Headers)
		}
		if agentRw.Name != nil {
			spec.Name = ptr.To(*agentRw.Name)
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// isFederationRelabelConfig returns true if a relabel config is designed to restore
// job and instance labels that were prefixed with "exported_" during PrometheusAgent's
// federated scraping (/federate).
//
// In raw metrics collection, metrics originate natively from the cluster's Prometheus
// (e.g. prometheus-k8s) and already retain their true "job" and "instance" labels.
// Propagating these rules into in-cluster remote_write configs causes Prometheus
// to overwrite and delete the native job and instance labels when "exported_job"
// is absent (since default replace regex is "(.*)" which matches empty strings).
func isFederationRelabelConfig(cfg cooprometheusv1.RelabelConfig) bool {
	for _, sl := range cfg.SourceLabels {
		if sl == "exported_job" || sl == "exported_instance" {
			return true
		}
	}
	if cfg.TargetLabel == "exported_job" || cfg.TargetLabel == "exported_instance" {
		return true
	}
	if strings.EqualFold(cfg.Action, "labeldrop") && (cfg.Regex == "exported_job|exported_instance" || cfg.Regex == "exported_instance|exported_job") {
		return true
	}
	return false
}
