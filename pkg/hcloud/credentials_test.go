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

package hcloud

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func secretWith(data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{Data: data}
}

func TestExtractToken(t *testing.T) {
	secret := secretWith(map[string][]byte{
		HcloudToken: []byte("common-value"),
	})

	t.Run("present key", func(t *testing.T) {
		tok, err := extractToken(secret, HcloudToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok == nil {
			t.Fatal("expected a token, got nil")
			return
		}
		if tok.Token != "common-value" {
			t.Fatalf("expected token %q, got %q", "common-value", tok.Token)
		}
	})

	t.Run("absent key", func(t *testing.T) {
		tok, err := extractToken(secret, HcloudTokenCCM)
		if err == nil {
			t.Fatal("expected an error for a missing key, got nil")
		}
		if tok != nil {
			t.Fatalf("expected nil token on error, got %+v", tok)
		}
		if !strings.Contains(err.Error(), HcloudTokenCCM) {
			t.Fatalf("expected error to mention %q, got %q", HcloudTokenCCM, err.Error())
		}
	})
}

func TestExtractCredentials(t *testing.T) {
	const (
		common = "common-tok"
		ccm    = "ccm-tok"
		csi    = "csi-tok"
		mcm    = "mcm-tok"
	)

	tests := []struct {
		name    string
		data    map[string][]byte // nil means Data is nil
		wantErr bool
		errHas  string
		// expected tokens returned by the accessors on success
		wantCCM string
		wantCSI string
		wantMCM string
	}{
		{
			name:    "nil Data",
			data:    nil,
			wantErr: true,
			errHas:  "secret does not contain any data",
		},
		{
			name:    "empty Data / all tokens missing",
			data:    map[string][]byte{},
			wantErr: true,
			errHas:  "need either common or cloud controller manager",
		},
		{
			name:    "common token only",
			data:    map[string][]byte{HcloudToken: []byte(common)},
			wantCCM: common,
			wantCSI: common,
			wantMCM: common,
		},
		{
			name: "all component tokens, no common",
			data: map[string][]byte{
				HcloudTokenCCM: []byte(ccm),
				HcloudTokenCSI: []byte(csi),
				HcloudTokenMCM: []byte(mcm),
			},
			wantCCM: ccm,
			wantCSI: csi,
			wantMCM: mcm,
		},
		{
			name: "common plus all component tokens (component wins)",
			data: map[string][]byte{
				HcloudToken:    []byte(common),
				HcloudTokenCCM: []byte(ccm),
				HcloudTokenCSI: []byte(csi),
				HcloudTokenMCM: []byte(mcm),
			},
			wantCCM: ccm,
			wantCSI: csi,
			wantMCM: mcm,
		},
		{
			name: "common plus CCM only (CSI/MCM fall back to common)",
			data: map[string][]byte{
				HcloudToken:    []byte(common),
				HcloudTokenCCM: []byte(ccm),
			},
			wantCCM: ccm,
			wantCSI: common,
			wantMCM: common,
		},
		{
			name: "common plus CSI only (CCM/MCM fall back to common)",
			data: map[string][]byte{
				HcloudToken:    []byte(common),
				HcloudTokenCSI: []byte(csi),
			},
			wantCCM: common,
			wantCSI: csi,
			wantMCM: common,
		},
		{
			name: "common plus MCM only (CCM/CSI fall back to common)",
			data: map[string][]byte{
				HcloudToken:    []byte(common),
				HcloudTokenMCM: []byte(mcm),
			},
			wantCCM: common,
			wantCSI: common,
			wantMCM: mcm,
		},
		{
			// No common token and a component token missing must be an error,
			// NOT a nil-pointer dereference: with only CCM set, CSI and MCM
			// have no fallback available.
			name:    "CCM only, no common (missing fallback => error, not panic)",
			data:    map[string][]byte{HcloudTokenCCM: []byte(ccm)},
			wantErr: true,
			errHas:  "need either common or container storage interface",
		},
		{
			name:    "CSI only, no common (missing fallback => error, not panic)",
			data:    map[string][]byte{HcloudTokenCSI: []byte(csi)},
			wantErr: true,
			errHas:  "need either common or cloud controller manager",
		},
		{
			name:    "MCM only, no common (missing fallback => error, not panic)",
			data:    map[string][]byte{HcloudTokenMCM: []byte(mcm)},
			wantErr: true,
			errHas:  "need either common or cloud controller manager",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creds, err := ExtractCredentials(secretWith(tc.data))

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil (creds=%+v)", creds)
				}
				if creds != nil {
					t.Fatalf("expected nil credentials on error, got %+v", creds)
				}
				if tc.errHas != "" && !strings.Contains(err.Error(), tc.errHas) {
					t.Fatalf("expected error to contain %q, got %q", tc.errHas, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if creds == nil {
				t.Fatal("expected credentials, got nil")
			}

			// The accessors must not panic and must return the expected token.
			// This exercises the fallback semantics; if the nil-guard in
			// ExtractCredentials regressed, a nil commonToken deref would panic
			// here instead of returning a value.
			if got := creds.CCM().Token; got != tc.wantCCM {
				t.Errorf("CCM(): expected %q, got %q", tc.wantCCM, got)
			}
			if got := creds.CSI().Token; got != tc.wantCSI {
				t.Errorf("CSI(): expected %q, got %q", tc.wantCSI, got)
			}
			if got := creds.MCM().Token; got != tc.wantMCM {
				t.Errorf("MCM(): expected %q, got %q", tc.wantMCM, got)
			}
		})
	}
}
