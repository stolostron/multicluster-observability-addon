package manifests

import (
	nfv1beta2 "github.com/netobserv/network-observability-operator/apis/flowcollector/v1beta2"
)

func buildFlowCollectorSpec(resources Options) (*nfv1beta2.FlowCollectorSpec, error) {
	fc := resources.FlowCollector
	// for _, secret := range resources.Secrets {
	// 	if err := templateWithSecret(&fc.Spec, secret); err != nil {
	// 		return nil, err
	// 	}
	// }

	// for _, configmap := range resources.ConfigMaps {
	// 	if err := templateWithConfigMap(&fc.Spec, configmap); err != nil {
	// 		return nil, err
	// 	}
	// }

	return &fc.Spec, nil
}
