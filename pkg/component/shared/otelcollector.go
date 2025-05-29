// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector"
)

// NewOtelCollector returns new OtelCollector deployer
func NewOtelCollector(
	c client.Client,
	namespace string,
) (
	component.DeployWaiter,
	error,
) {
	deployer := collector.New(c, namespace, collector.Values{Image: ""})

	return deployer, nil
}
