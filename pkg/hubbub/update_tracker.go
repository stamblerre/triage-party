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

package hubbub

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/google/triage-party/pkg/provider"

	"k8s.io/klog/v2"
)

// mtime is a workaround the GitHub misfeature that UpdatedAt is not incremented for cross-reference events
func (h *Engine) mtime(i provider.IItem) time.Time {
	return h.mtimeKey(i.GetUpdatedAt(), updateKey(i))
}

// mtimeCo is like mtime, but for conversations
func (h *Engine) mtimeCo(co *Conversation) time.Time {
	return h.mtimeKey(co.Updated, fmt.Sprintf("%s/%s#%d", co.Organization, co.Project, co.ID))
}

// mtimeRef is like mtime, but for related conversations
func (h *Engine) mtimeRef(rc *RelatedConversation) time.Time {
	return h.mtimeKey(rc.Updated, fmt.Sprintf("%s/%s#%d", rc.Organization, rc.Project, rc.ID))
}

func (h *Engine) updatedAt(url string) time.Time {
	result, ok := h.updated.Load(url)
	if !ok {
		return time.Time{}
	}

	return result.(time.Time)
}

func (h *Engine) mtimeKey(idea time.Time, key string) time.Time {
	updatedAt := idea
	updateSeen := h.updatedAt(key)
	klog.V(2).Infof("%s was definitely updated by %s - possibly by %s", key, updatedAt, updateSeen)

	if updateSeen == updatedAt {
		return updatedAt
	}

	if updateSeen.After(updatedAt) {
		return updateSeen
	}

	return updatedAt
}

func updateKey(i provider.IItem) string {
	// https://github.com/kubernetes/minikube/pull/8431
	parts := strings.Split(i.GetHTMLURL(), "/")
	if len(parts) < 7 {
		klog.Errorf("unexpected URL: %s -> %v", i.GetHTMLURL(), parts)
		return ""
	}

	num := parts[len(parts)-1]
	project := parts[len(parts)-3]
	org := parts[len(parts)-4]
	return fmt.Sprintf("%s/%s#%s", org, project, num)
}

func (h *Engine) updateMtime(i provider.IItem, t time.Time) {
	key := updateKey(i)
	h.updateMtimeByKey(key, t)
}

func (h *Engine) updateCoMtime(co *Conversation, t time.Time) {
	key := fmt.Sprintf("%s/%s#%d", co.Organization, co.Project, co.ID)
	h.updateMtimeByKey(key, t)
}

func (h *Engine) updateMtimeLong(org string, project string, num int, t time.Time) {
	key := fmt.Sprintf("%s/%s#%d", org, project, num)
	h.updateMtimeByKey(key, t)
}

func (h *Engine) updateMtimeByKey(key string, ts time.Time) {
	if ts.After(h.updatedAt(key)) {
		if !h.updatedAt(key).IsZero() {
			_, file, no, ok := runtime.Caller(2)
			if ok {
				klog.V(2).Infof("Updating %s last update time for %s to %s - caller: %s:%d", key, h.updatedAt(key), ts, file, no)
			} else {
				klog.V(2).Infof("Updating %s last update time for %s to %s", key, h.updatedAt(key), ts)
			}
		}
		h.updated.Store(key, ts)
	}
}
