# Wrapper around grafana/grafana: adds the `service` label Kamal's
# `validate_image` requires and bakes in provisioning and dashboards
# (infra/grafana/dashboards/ is the source of truth).
FROM grafana/grafana:13.2.4

# GitHub and Sentry datasources, wired up in provisioning/datasources/issue-signals.yml.
# They hold the GitHub PAT and Sentry token, so they're pinned and installed at
# build time; Grafana never fetches them at startup.
# renovate: datasource=custom.grafana-plugin depName=grafana-github-datasource
ARG GRAFANA_GITHUB_DATASOURCE_VERSION=2.9.4
# renovate: datasource=custom.grafana-plugin depName=grafana-sentry-datasource
ARG GRAFANA_SENTRY_DATASOURCE_VERSION=2.2.9
RUN grafana cli --pluginsDir "$GF_PATHS_PLUGINS" plugins install grafana-github-datasource "$GRAFANA_GITHUB_DATASOURCE_VERSION" \
  && grafana cli --pluginsDir "$GF_PATHS_PLUGINS" plugins install grafana-sentry-datasource "$GRAFANA_SENTRY_DATASOURCE_VERSION"

# Grafana reads these paths on startup without any GF_PATHS_* override.
COPY infra/grafana/provisioning/ /etc/grafana/provisioning/
COPY infra/grafana/dashboards/ /var/lib/grafana/dashboards/
