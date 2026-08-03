/*
Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package apis is the main package for HCloud specific APIs
package apis

import (
	"os"
	"sync"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

// singletonsMutex guards singletons: reconcilers read it concurrently while
// SetClientForToken may write, and Go maps are not safe for concurrent read/write.
var (
	singletonsMutex sync.RWMutex
	singletons      = make(map[string]*hcloud.Client)
)

// GetClientForToken returns an underlying HCloud client for the given token.
//
// A client preregistered via SetClientForToken is returned as-is; otherwise a
// fresh one is built per call. Deliberately NOT cached: hcloud.Client keeps the
// plaintext token in a field, so caching would pin every token the process ever
// saw — including rotated and revoked ones — in the heap for its whole lifetime,
// with no eviction path. There is nothing to gain in exchange: hcloud.NewClient
// leaves http.Client.Transport nil, so every client already shares
// http.DefaultTransport's connection pool, and the rate-limit handler is
// stateless (it only reads response headers).
//
// PARAMETERS
// token string Token to look up client instance for
func GetClientForToken(token string) *hcloud.Client {
	singletonsMutex.RLock()
	client, ok := singletons[token]
	singletonsMutex.RUnlock()

	if ok {
		return client
	}

	opts := []hcloud.ClientOption{
		hcloud.WithToken(token),
		hcloud.WithApplication("gardener-extension-provider-hcloud", "v0.0.0"),
	}
	if endpoint := os.Getenv("HCLOUD_ENDPOINT"); endpoint != "" {
		opts = append(opts, hcloud.WithEndpoint(endpoint))
	}

	return hcloud.NewClient(opts...)
}

// SetClientForToken sets a preconfigured HCloud client for the given token.
//
// PARAMETERS
// token  string         Token to look up client instance for
// client *hcloud.Client Preconfigured HCloud client
func SetClientForToken(token string, client *hcloud.Client) {
	singletonsMutex.Lock()
	defer singletonsMutex.Unlock()

	if client == nil {
		delete(singletons, token)
	} else {
		singletons[token] = client
	}
}
