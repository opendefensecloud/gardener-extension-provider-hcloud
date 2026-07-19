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

// Package infrastructure contains functions used at the infrastructure controller
package infrastructure

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendefensecloud/gardener-extension-provider-hcloud/pkg/hcloud/apis/mock"
)

// expectSecretGet registers an expectation for the credentials secret lookup
// performed by getActuatorConfig. It returns a secret carrying the mock token
// so that apis.GetClientForToken hands back the httptest-backed HCloud client
// wired up in the suite's BeforeSuite.
func expectSecretGet(times int) {
	mockTestEnv.Client.EXPECT().
		Get(gomock.Any(), k8sclient.ObjectKey{Namespace: mock.TestNamespace, Name: mock.TestInfrastructureSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
		DoAndReturn(func(_ context.Context, _ k8sclient.ObjectKey, secret *corev1.Secret, _ ...k8sclient.GetOption) error {
			secret.Data = map[string][]byte{
				"hcloudToken": []byte("dummy-token"),
			}
			return nil
		}).
		Times(times)
}

// newInfrastructureWithStatus returns the standard test infrastructure augmented
// with the given provider status raw JSON so that the delete path can decode a
// populated InfrastructureStatus.
func newInfrastructureWithStatus(statusJSON string) *extensionsv1alpha1.Infrastructure {
	infra := mock.NewInfrastructure()
	infra.Status.ProviderStatus = &runtime.RawExtension{
		Raw: []byte(statusJSON),
	}
	return infra
}

var _ = Describe("ActuatorDelete", func() {
	Describe("#Delete", func() {
		It("should successfully delete when no infrastructure status is present", func() {
			// With an empty status the decoded InfrastructureStatus carries no
			// NetworkIDs and no SSHFingerprint, so both ensurer deletions are
			// no-ops and updateProviderStatus(nil) returns without patching.
			expectSecretGet(1)

			err := infraActuator.Delete(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully tear down networks and ssh key when neither resource exists remotely", func() {
			// NetworkIDs.Workers and a (non-matching) SSHFingerprint drive both
			// ensurer deletion calls, but the mock HCloud endpoints report the
			// resources as absent, so no DELETE requests are issued.
			statusJSON := `{
				"apiVersion": "hcloud.provider.extensions.gardener.cloud/v1alpha1",
				"kind": "InfrastructureStatus",
				"sshFingerprint": "00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:00",
				"networkIDs": {"workers": "42"}
			}`
			expectSecretGet(1)

			err := infraActuator.Delete(ctx, logr.Logger{}, newInfrastructureWithStatus(statusJSON), cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error when the ssh key deletion request fails", func() {
			// A fingerprint matching the mock's /ssh_keys listing resolves to an
			// existing key (id 42). Deleting it issues DELETE /ssh_keys/42 which
			// has no registered handler and therefore fails - exercising the
			// partial-teardown failure branch of delete.
			statusJSON := fmt.Sprintf(`{
				"apiVersion": "hcloud.provider.extensions.gardener.cloud/v1alpha1",
				"kind": "InfrastructureStatus",
				"sshFingerprint": "%s"
			}`, mock.TestSSHFingerprint)
			expectSecretGet(1)

			err := infraActuator.Delete(ctx, logr.Logger{}, newInfrastructureWithStatus(statusJSON), cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error when the credentials secret cannot be read", func() {
			mockTestEnv.Client.EXPECT().
				Get(gomock.Any(), k8sclient.ObjectKey{Namespace: mock.TestNamespace, Name: mock.TestInfrastructureSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(fmt.Errorf("secret not found")).
				Times(1)

			err := infraActuator.Delete(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#Reconcile error cleanup", func() {
		It("should run the cleanup and surface the error when the status update fails", func() {
			// reconcile succeeds through the ensurer calls but fails when
			// persisting the provider status. Reconcile must then invoke
			// reconcileOnErrorCleanup (which re-reads the actuator config) and
			// return the original error.
			expectSecretGet(2)
			mockTestEnv.Client.EXPECT().Status().Return(sw).AnyTimes()
			sw.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("patch failed")).Times(1)

			err := infraActuator.Reconcile(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should surface the error and skip cleanup when the actuator config cannot be built", func() {
			// The secret lookup fails, so reconcile aborts in getActuatorConfig
			// and reconcileOnErrorCleanup takes its nil-config early-exit path.
			// Get is invoked once by reconcile and once by the cleanup.
			mockTestEnv.Client.EXPECT().
				Get(gomock.Any(), k8sclient.ObjectKey{Namespace: mock.TestNamespace, Name: mock.TestInfrastructureSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				Return(fmt.Errorf("secret not found")).
				Times(2)

			err := infraActuator.Reconcile(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		It("should be a no-op and return nil (pins current behaviour)", func() {
			// NOTE: Restore is currently a no-op - it performs no restoration and
			// unconditionally returns nil. This test documents today's behaviour
			// so that any future change is caught deliberately.
			err := infraActuator.Restore(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Migrate", func() {
		It("should be a no-op and return nil (pins current behaviour)", func() {
			err := infraActuator.Migrate(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ForceDelete", func() {
		It("should be a no-op and return nil (pins current behaviour)", func() {
			err := infraActuator.ForceDelete(ctx, logr.Logger{}, mock.NewInfrastructure(), cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
