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
	client       client.Client
	namespace    string
	values       Values
	lokiEndpoint string
}

// New creates a new instance of otel-collector deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
	lokiEndpoint string,
) component.DeployWaiter {
	return &otelCollector{
		client:       client,
		namespace:    namespace,
		values:       values,
		lokiEndpoint: lokiEndpoint,
	}
}

func (f *otelCollector) Deploy(ctx context.Context) error {
	var (
		registry  = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		collector = f.openTelemetryCollector(f.namespace, f.lokiEndpoint)
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

func (f *otelCollector) openTelemetryCollector(namespace, lokiEndpoint string) *otelv1beta1.OpenTelemetryCollector {
	obj := &otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameOpenTelemetryCollector,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Mode:            "deployment",
			UpgradeStrategy: "none",
			OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
				Image: "docker.io/otel/opentelemetry-collector-contrib:0.115.1",
			},
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]interface{}{
						"loki": map[string]interface{}{
							"protocols": map[string]interface{}{
								"http": map[string]interface{}{
									"endpoint": "0.0.0.0:4317",
								},
							},
						},
					},
				},
				//resource/journal:
				// attributes:
				//   - action: insert
				//     key: origin
				//     value: systemd-journal
				//   - key: loki.resource.labels
				//     value: unit, nodename, origin
				//     action: insert
				//   - key: loki.format
				//     value: logfmt
				//     action: insert
				//
				Processors: &otelv1beta1.AnyConfig{
					Object: map[string]interface{}{
						"batch": map[string]interface{}{
							"timeout": "10s",
						},
						"resource/labels": map[string]interface{}{
							"attributes": []map[string]interface{}{
								{
									"key":    "loki.resource.labels",
									"value":  "unit, nodename, origin, pod_name, container_name, origin, namespace_name, nodename, gardener_cloud_role",
									"action": "insert",
								},
								{
									"key":    "loki.format",
									"value":  "logfmt",
									"action": "insert",
								},
							},
						},
					},
				},
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]interface{}{
						"debug": map[string]interface{}{
							"verbosity": "detailed",
						},
						"loki": map[string]interface{}{
							"endpoint": lokiEndpoint,
						},
					},
				},
				Service: otelv1beta1.Service{
					Pipelines: map[string]*otelv1beta1.Pipeline{
						"logs": {
							Exporters:  []string{"debug", "loki"},
							Receivers:  []string{"loki"},
							Processors: []string{"resource/labels", "batch"},
						},
					},
					Telemetry: &otelv1beta1.AnyConfig{
						Object: map[string]interface{}{
							"logs": map[string]interface{}{
								"level": "debug",
							},
						},
					},
				},
			},
		},
	}

	// utilruntime.Must(references.InjectAnnotations(obj))
	return obj
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.resources.gardener.cloud/to-all-shoots-logging-tcp-3100":                v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.gardener.cloud/to-dns":                                                  v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.gardener.cloud/to-runtime-apiserver":                                    v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:                                      "opentelemetry-collector",
	}
}
