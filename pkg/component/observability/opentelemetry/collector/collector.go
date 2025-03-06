// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"time"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName     = "opentelemetry-collector"
	otelCollectorConfigName = "opentelemetry-collector-config"
)

// Values is the values for otel-collector configurations
type Values struct {
	// Image is the collector image.
	Image string
}

type otelCollector struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of otel-collector deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &otelCollector{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (f *otelCollector) Deploy(ctx context.Context) error {
	var (
		registry  = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		collector = openTelemetryCollector(f.namespace)
	)

	resources := []client.Object{collector}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, f.client, f.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (f *otelCollector) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, f.client, f.namespace, managedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (f *otelCollector) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, f.client, f.namespace, managedResourceName)
}

func (f *otelCollector) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, f.client, f.namespace, managedResourceName)
}

func openTelemetryCollector(namespace string) *otelv1beta1.OpenTelemetryCollector {
	return &otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameOpenTelemetryCollector,
			Namespace: namespace,
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Mode:            "deployment",
			UpgradeStrategy: "none",
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]interface{}{
						"otlp": map[string]interface{}{
							"protocols": map[string]interface{}{
								"grpc": map[string]interface{}{
									"endpoint": "0.0.0.0:4317",
								},
							},
						},
					},
				},
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]interface{}{
						"debug": map[string]interface{}{},
					},
				},
				Service: otelv1beta1.Service{
					Pipelines: map[string]*otelv1beta1.Pipeline{
						"traces": {
							Exporters: []string{"debug"},
							Receivers: []string{"otlp"},
						},
					},
				},
			},
		},
	}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                             v1beta1constants.DaemonSetNameFluentBit,
		v1beta1constants.LabelRole:                            v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleLogging,
		v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.resources.gardener.cloud/to-all-shoots-logging-tcp-3100":                v1beta1constants.LabelNetworkPolicyAllowed,
	}
}

func getCustomResourcesLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource,
	}
}
