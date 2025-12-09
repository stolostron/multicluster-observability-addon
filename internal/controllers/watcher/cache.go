package watcher

import (
	"fmt"
	"sync"
)

type ReferenceCache struct {
	sync.RWMutex
	// configToMWNs maps a Configuration Resource Key (Group/Kind/Namespace/Name)
	// to a set of ManifestWork Namespaces that reference it.
	configToMWNs map[string]map[string]struct{}
	// mwKeyToConfigs maps a ManifestWork Key (Namespace/Name) to the set of
	// Configuration Resource Keys it references. Used for efficient cleanup.
	mwKeyToConfigs map[string]map[string]struct{}
}

func NewReferenceCache() *ReferenceCache {
	return &ReferenceCache{
		configToMWNs:   make(map[string]map[string]struct{}),
		mwKeyToConfigs: make(map[string]map[string]struct{}),
	}
}

func (c *ReferenceCache) Add(mwNamespace, mwName string, configKeys []string) {
	c.Lock()
	defer c.Unlock()

	mwKey := fmt.Sprintf("%s/%s", mwNamespace, mwName)
	newConfigs := make(map[string]struct{})
	for _, k := range configKeys {
		newConfigs[k] = struct{}{}
	}

	// Remove old references if any (handle update)
	if oldConfigs, exists := c.mwKeyToConfigs[mwKey]; exists {
		for configKey := range oldConfigs {
			if _, keep := newConfigs[configKey]; !keep {
				c.removeRef(mwNamespace, configKey)
			}
		}
	}

	// Add new references
	c.mwKeyToConfigs[mwKey] = newConfigs
	for _, configKey := range configKeys {
		if _, ok := c.configToMWNs[configKey]; !ok {
			c.configToMWNs[configKey] = make(map[string]struct{})
		}
		c.configToMWNs[configKey][mwNamespace] = struct{}{}
	}
}

func (c *ReferenceCache) Remove(mwNamespace, mwName string) {
	c.Lock()
	defer c.Unlock()

	mwKey := fmt.Sprintf("%s/%s", mwNamespace, mwName)
	if oldConfigs, exists := c.mwKeyToConfigs[mwKey]; exists {
		for configKey := range oldConfigs {
			c.removeRef(mwNamespace, configKey)
		}
		delete(c.mwKeyToConfigs, mwKey)
	}
}

func (c *ReferenceCache) removeRef(mwNamespace, configKey string) {
	if nss, ok := c.configToMWNs[configKey]; ok {
		delete(nss, mwNamespace)
		if len(nss) == 0 {
			delete(c.configToMWNs, configKey)
		}
	}
}

func (c *ReferenceCache) GetNamespaces(configKey string) []string {
	c.RLock()
	defer c.RUnlock()

	var result []string
	if nss, ok := c.configToMWNs[configKey]; ok {
		for ns := range nss {
			result = append(result, ns)
		}
	}
	return result
}
