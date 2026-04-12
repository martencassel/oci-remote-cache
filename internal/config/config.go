package config

import (
	"fmt"
	"sync"
)

type RepoConfig struct {
	RepoKey   string
	RemoteURL string
}

type RepoConfigProvider interface {
	GetRepoConfig(repoKey string) (*RepoConfig, error)
}

type repoConfigInMemory struct {
	configs map[string]*RepoConfig
	mu      sync.RWMutex
}

func NewRepoConfigInMemory() *repoConfigInMemory {
	return &repoConfigInMemory{
		configs: make(map[string]*RepoConfig),
	}
}

func (p *repoConfigInMemory) GetRepoConfig(repoKey string) (*RepoConfig, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	config, exists := p.configs[repoKey]
	if !exists {
		return nil, fmt.Errorf("repository config not found for key: %s", repoKey)
	}
	return config, nil
}

func (p *repoConfigInMemory) AddRepoConfig(config *RepoConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.configs[config.RepoKey] = config
}

func (p *repoConfigInMemory) RemoveRepoConfig(repoKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.configs, repoKey)
}
