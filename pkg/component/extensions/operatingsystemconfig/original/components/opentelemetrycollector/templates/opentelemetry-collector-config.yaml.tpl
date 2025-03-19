extensions:
  file_storage:
    directory: /var/log/otelcol
    create_directory: true

receivers:
  journald/journal:
    start_at: beginning
    storage: file_storage

  filelog/pods:
    include: [{{range .shootComponents}}/var/log/pods/kube-system_{{.}}*/*/*.log,{{end}}]
    storage: file_storage
    include_file_path: true
    operators:
      - type: container
        format: containerd
        add_metadata_from_filepath: true

processors:
  batch:
    timeout: 10s

  resourcedetection/system:
    detectors: ["system"]
    system:
      hostname_sources: ["os"]

  filter/drop_localhost_journal:
    logs:
      exclude:
        match_type: strict
        resource_attributes:
          - key: _HOSTNAME
            value: localhost

  filter/keep_units_journal:
    logs:
      include:
        match_type: strict
        resource_attributes:
          - key: SYSLOG_IDENTIFIER
            value: kernel
          - key: _SYSTEMD_UNIT
            value: kubelet.service
          - key: _SYSTEMD_UNIT
            value: docker.service
          - key: _SYSTEMD_UNIT
            value: containerd.service
          - key: _SYSTEMD_UNIT
            value: gardener-node-agent.service

  filter/drop_units_combine:
    logs:
      exclude:
        match_type: strict
        resource_attributes:
          - key: SYSLOG_IDENTIFIER
            value: kernel
          - key: _SYSTEMD_UNIT
            value: kubelet.service
          - key: _SYSTEMD_UNIT
            value: docker.service
          - key: _SYSTEMD_UNIT
            value: containerd.service
          - key: _SYSTEMD_UNIT
            value: gardener-node-agent.service

  attributes/journal_labels:
    actions:
      - key: unit
        from_attribute: SYSLOG_IDENTIFIER
        action: insert
      - key: nodename
        from_attribute: _HOSTNAME
        action: insert

  attributes/combine_labels:
    actions:
      - key: unit
        from_attribute: SYSLOG_IDENTIFIER
        action: insert
      - key: nodename
        from_attribute: _HOSTNAME
        action: insert

  resource/journal:
    attributes:
      - action: insert
        key: job
        value: systemd-journal
      - action: insert
        key: origin
        value: systemd-journal

  resource/combine_journal:
    attributes:
      - action: insert
        key: job
        value: systemd-combine-journal
      - action: insert
        key: origin
        value: systemd-journal

  resource/pod_labels:
    attributes:
      - key: nodename
        from_attribute: k8s.pod.node.name
        action: insert
      - key: namespace_name
        value: "kube-system"
        action: insert
      - key: pod_name
        from_attribute: k8s.pod.name
        action: insert
      - key: container_name
        from_attribute: k8s.container.name
        action: insert
      - key: origin
        value: "shoot_system"
        action: upsert
      - key: loki.resource.labels
        value: pod_name
        action: upsert

  filter/keep_gardener:
    logs:
      include:
        match_type: strict
        resource_attributes:
          - key: resource["k8s.pod.labels.origin"]
            value: "gardener"

exporters:
  loki:
    endpoint: {{ .clientURL }}
    headers:
      Authorization: "Bearer ${file:{{ .pathAuthToken }}}"
    tls:
      ca_file: {{ .pathCACert }}

  debug:
    verbosity: detailed

service:
  extensions: [file_storage]
  pipelines:
    logs/journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/keep_units_journal, attributes/journal_labels, resource/journal, batch]
      exporters: [loki]
    logs/combine_journal:
      receivers: [journald/journal]
      processors: [filter/drop_localhost_journal, filter/drop_units_combine, attributes/combine_labels, resource/combine_journal, batch]
      exporters: [loki]
    logs/pods:
      receivers: [filelog/pods]
      processors: [resourcedetection/system, resource/pod_labels]
      exporters: [loki, debug]
