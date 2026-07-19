// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"strings"
	"testing"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const containerdPinPath = "/etc/apt/preferences.d/gardener-containerd-pin"

func TestAddContainerdAptPinFile(t *testing.T) {
	var files []extensionsv1alpha1.File
	addContainerdAptPinFile(&files)

	if len(files) != 1 {
		t.Fatalf("expected exactly one file, got %d", len(files))
	}
	f := files[0]

	if f.Path != containerdPinPath {
		t.Errorf("path: got %q, want %q", f.Path, containerdPinPath)
	}
	if f.Permissions == nil || *f.Permissions != 0644 {
		t.Errorf("permissions: got %v, want 0644", f.Permissions)
	}
	if f.Content.Inline == nil {
		t.Fatal("expected inline file content")
	}
	content := f.Content.Inline.Data

	// The pin must be version-conditioned: refuse only the broken 2.2.0/2.2.1
	// (Pin-Priority < 0 => never installed) so 22.04/24.04 fall back to 1.7.x and
	// 26.04+ can use 2.2.2+.
	mustContain := []string{
		"Package: containerd",
		"Pin: version 2.2.0-*",
		"Pin: version 2.2.1-*",
		"Pin-Priority: -1",
	}
	for _, want := range mustContain {
		if !strings.Contains(content, want) {
			t.Errorf("pin content missing %q; got:\n%s", want, content)
		}
	}

	// It must NOT force a release: no 1.7.x pin and no priority-1001 override,
	// which would break 26.04 (no 1.7.x at all) and wrongly force downgrades.
	mustNotContain := []string{
		"1.7.*",
		"Pin-Priority: 1001",
	}
	for _, unwanted := range mustNotContain {
		if strings.Contains(content, unwanted) {
			t.Errorf("pin content must not contain %q; got:\n%s", unwanted, content)
		}
	}

	// Two separate blocking stanzas (one per broken version).
	if got := strings.Count(content, "Pin-Priority: -1"); got != 2 {
		t.Errorf("expected 2 blocking stanzas (Pin-Priority: -1), got %d", got)
	}
}

func TestAddContainerdAptPinFileIsIdempotent(t *testing.T) {
	var files []extensionsv1alpha1.File
	addContainerdAptPinFile(&files)
	addContainerdAptPinFile(&files)

	if len(files) != 1 {
		t.Fatalf("expected the pin file to be de-duplicated by path, got %d files", len(files))
	}
	if files[0].Path != containerdPinPath {
		t.Errorf("path: got %q, want %q", files[0].Path, containerdPinPath)
	}
}
