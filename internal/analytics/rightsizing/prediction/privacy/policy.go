package privacy

import (
	"net"
	"net/url"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateNetworkPolicy builds egress rules suited to the prediction provider type.
func GenerateNetworkPolicy(namespace, providerType, customEndpoint string) *networkingv1.NetworkPolicy {
	const name = "rs-prediction-netpol"
	base := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}

	switch providerType {
	case "builtin", "onnx":
		// Deny egress: no rules other than policy type egress.
		base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{}
	case "external":
		// Allow all egress; customer endpoint is unknown at IP level.
		base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
			{},
		}
	case "custom":
		if strings.Contains(customEndpoint, "svc.cluster.local") {
			base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{}
			break
		}
		host := endpointHost(customEndpoint)
		if host == "" {
			base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{}}
			break
		}
		if ip := net.ParseIP(host); ip != nil {
			var cidr string
			if ip.To4() != nil {
				cidr = ip.String() + "/32"
			} else {
				cidr = ip.String() + "/128"
			}
			base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{IPBlock: &networkingv1.IPBlock{CIDR: cidr}},
					},
				},
			}
			break
		}
		// Hostname without resolvable IP at policy creation time: allow non-RFC1918 egress broadly.
		base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
			{
				To: []networkingv1.NetworkPolicyPeer{
					{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
					{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
				},
			},
		}
	default:
		base.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{}
	}

	return base
}

func endpointHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Hostname()
	return host
}
