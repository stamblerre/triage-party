// Copyright 2020 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// hubbub provides an advanced in-memory search for GitHub using state machines
package hubbub

import (
	"sync"
	"time"

	"github.com/google/triage-party/pkg/constants"
	"github.com/google/triage-party/pkg/persist"
	"github.com/google/triage-party/pkg/provider"
	"k8s.io/klog/v2"
)

// Config is how to configure a new hubbub engine
type Config struct {
	Cache persist.Cacher // Cacher is a cache interface
	Repos []string       // Repos is the repositories to search

	// MinSimilarity is how close two items need to be to each other to be called similar
	MinSimilarity float64

	// The furthest we will query back for information on closed issues
	MaxClosedUpdateAge time.Duration

	// DebugNumbers is used when you want to debug why a single item is being handled in a certain wait
	DebugNumbers map[int]bool

	// MemberRoles are which roles to consider as members
	// https://developer.github.com/v4/enum/commentauthorassociation/
	MemberRoles []string

	// Members are which specific users to consider as members
	Members []string

	// Providers
	GitHub provider.Provider
	GitLab provider.Provider
}

// Engine is the search engine interface for hubbub
type Engine struct {
	cache persist.Cacher

	// Must be settable from config
	MinSimilarity float64

	// The furthest we will query back for information on closed issues
	MaxClosedUpdateAge time.Duration

	debug map[int]bool

	titleToURLs   sync.Map
	similarTitles sync.Map

	memberRoles map[string]bool
	members     map[string]bool

	// Data source providers
	github provider.Provider
	gitlab provider.Provider

	// Workaround because GitHub doesn't update issues if cross-references occur
	updated sync.Map

	// indexes used for similarity matching & conversation caching
	seen sync.Map
}

// ConversationsTotal returns the number of conversations we've seen so far
func (e *Engine) ConversationsTotal() int {
	t := 0
	e.seen.Range(func(key, value interface{}) bool {
		t++
		return true
	})

	return t
}

func (e *Engine) provider(hostname string) provider.Provider {
	if hostname == constants.GitLabProviderHost {
		return e.gitlab
	}
	return e.github
}

func New(cfg Config) *Engine {
	e := &Engine{
		cache: cfg.Cache,

		MaxClosedUpdateAge: cfg.MaxClosedUpdateAge,
		MinSimilarity:      cfg.MinSimilarity,
		debug:              cfg.DebugNumbers,

		memberRoles: map[string]bool{},
		members:     map[string]bool{},

		github: cfg.GitHub,
		gitlab: cfg.GitLab,
	}

	klog.Infof("considering users as members: %v", cfg.Members)
	for _, user := range cfg.Members {
		e.members[user] = true
	}

	klog.Infof("considering roles as members: %v", cfg.MemberRoles)
	for _, role := range cfg.MemberRoles {
		e.memberRoles[role] = true
	}

	if len(e.members) == 0 && len(e.memberRoles) == 0 {
		e.memberRoles = map[string]bool{"collaborator": true, "member": true, "owner": true}
		klog.Warningf("No memberships defined, using default: %v", e.memberRoles)
	}

	// This value is typically programmed on the fly, but lets give it a good enough default
	if e.MaxClosedUpdateAge == 0 {
		e.MaxClosedUpdateAge = 24 * 3 * time.Hour
	}

	return e
}
