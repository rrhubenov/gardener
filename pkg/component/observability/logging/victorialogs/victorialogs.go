// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package victorialogs

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	victorialogsconstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName            = "victorialogs"
	timeoutWaitForManagedResources = 2 * time.Minute
)

// Values is the values for VictoriaLogs configurations
type Values struct {
	// Image is the VictoriaLogs image.
	Image string
}

type victoriaLogs struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of VictoriaLogs deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &victoriaLogs{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (v *victoriaLogs) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	serializedResources, err := registry.AddAllAndSerialize(v.vlSingle())
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, v.client, v.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (v *victoriaLogs) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, v.client, v.namespace, managedResourceName)
}

func (v *victoriaLogs) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *victoriaLogs) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *victoriaLogs) vlSingle() *vmv1.VLSingle {
	return &vmv1.VLSingle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      victorialogsconstants.VLSingleResourceName,
			Namespace: v.namespace,
			Labels:    getLabels(),
		},
		Spec: vmv1.VLSingleSpec{
			CommonDefaultableParams: vmv1beta1.CommonDefaultableParams{
				Image: vmv1beta1.Image{
					Repository: v.values.Image,
				},
				Port: "9428",
			},
		},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToDNS:                                            v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                               v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:                                      "victorialogs",
	}
}
