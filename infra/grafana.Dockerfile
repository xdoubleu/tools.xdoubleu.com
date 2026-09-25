# Wrapper around grafana/grafana: adds the `service` label Kamal's
# `validate_image` requires and bakes in provisioning and dashboards
# (infra/grafana/dashboards/ is the source of truth).
FROM grafana/grafana:13.2.2

# GitHub and Sentry datasources, wired up in provisioning/datasources/issue-signals.yml.
ENV GF_INSTALL_PLUGINS=grafana-github-datasource,grafana-sentry-datasource

# Grafana reads these paths on startup without any GF_PATHS_* override.
COPY infra/grafana/provisioning/ /etc/grafana/provisioning/
COPY infra/grafana/dashboards/ /var/lib/grafana/dashboards/
