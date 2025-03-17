// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetrycollector

import (
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
)

const (
	// UnitName is the name of the opentelemetry-collector service.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameOpenTelemetryCollector

	// PathDirectory is the path for the opentelemetry-collector's directory.
	PathDirectory = "/var/lib/opentelemetry-collector"
	// PathAuthToken is the path for the file containing opentelemetry-collector's authentication token for communication with the Vali
	// sidecar proxy.
	PathAuthToken = PathDirectory + "/auth-token"
	// PathConfig is the path for the opentelemetry-collector's configuration file.
	PathConfig = v1beta1constants.OperatingSystemConfigFilePathOpenTelemetryCollector
	// PathCACert is the path for the vali-tls certificate authority.
	PathCACert = PathDirectory + "/ca.crt"

	opentelemetryCollectorBinaryPath = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/opentelemetry-collector"
)

type component struct{}

// New returns a new opentelemetry-collector component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "opentelemetry-collector"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var (
		units []extensionsv1alpha1.Unit
		files []extensionsv1alpha1.File
	)

	if ctx.ValitailEnabled {
		collectorConfigFile, err := getOpentelemetryCollectorConfigurationFile(ctx)
		if err != nil {
			return nil, nil, err
		}

		units = append(units, getOpenTelemetryCollectorUnit())
		files = append(files, collectorConfigFile, getOpenTelemetryCollectorCAFile(ctx), extensionsv1alpha1.File{
			Path:        opentelemetryCollectorBinaryPath,
			Permissions: ptr.To[uint32](0755),
			Content: extensionsv1alpha1.FileContent{
				ImageRef: &extensionsv1alpha1.FileContentImageRef{
					Image: ctx.Images[imagevector.ContainerImageNameOpentelemetryCollector].String(),
					// TODO(rado): Update value to the actual otel binary in the container
					FilePathInImage: "/usr/bin/valitail",
				},
			},
		})
	}

	return units, files, nil
}
