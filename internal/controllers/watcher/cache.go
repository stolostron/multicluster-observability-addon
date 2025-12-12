package watcher

import (
	"fmt"
	"maps"
	"sync"
)

type ReferenceCache struct {
	sync.RWMutex
	// configToMWNs maps a Configuration Resource Key (Group/Kind/Namespace/Name)
	// to a set of ManifestWork Namespaces that reference it.
	// NOTE: This assumes that a given configuration resource is referenced by at most
	// one ManifestWork in a given namespace. If multiple ManifestWorks in the same
	// namespace reference the same configuration, removing one will stop tracking
	// for the entire namespace.
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

func (c *ReferenceCache) Add(mwNamespace, mwName string, newConfigs map[string]struct{}) {
	mwKey := fmt.Sprintf("%s/%s", mwNamespace, mwName)

	c.RLock()
	oldConfigs, exists := c.mwKeyToConfigs[mwKey]
	if exists && maps.Equal(oldConfigs, newConfigs) {
		c.RUnlock()
		return
	}
	c.RUnlock()

	c.Lock()
	defer c.Unlock()

	// Re-check after acquiring write lock to handle potential race conditions
	oldConfigs, exists = c.mwKeyToConfigs[mwKey]
	if exists && maps.Equal(oldConfigs, newConfigs) {
		return
	}

	// Remove old references if any (handle update)
	if exists {
		for configKey := range oldConfigs {
			if _, keep := newConfigs[configKey]; !keep {
				c.removeRef(mwNamespace, configKey)
			}
		}
	}

	// Add new references
	c.mwKeyToConfigs[mwKey] = newConfigs
	for configKey := range newConfigs {
		if _, ok := c.configToMWNs[configKey]; !ok {
			c.configToMWNs[configKey] = make(map[string]struct{})
		}
		c.configToMWNs[configKey][mwNamespace] = struct{}{}
	}
}

func (c *ReferenceCache) Remove(mwNamespace, mwName string) {
	mwKey := fmt.Sprintf("%s/%s", mwNamespace, mwName)

	c.RLock()
	_, exists := c.mwKeyToConfigs[mwKey]
	c.RUnlock()

	if !exists {
		return
	}

	c.Lock()
	defer c.Unlock()

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
