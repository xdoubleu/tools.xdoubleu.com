# Thin wrapper around the stock grafana/grafana-oss image (issue #1509).
# Originally existed purely so build-grafana.yml could stamp the `service`
# docker label Kamal's `validate_image` step requires on every deployed
# image (see build-api.yml's identical comment) — the upstream image
# carries no such label. It now also bakes in the datasource + dashboard
# provisioning (issue #1527) so Grafana comes up fully configured instead
# of click-configured: the JSON under infra/grafana/dashboards/ is the
# single source of truth (allowUiUpdates:false in the provider config), and
# a change there triggers a fresh image build the same way a Dockerfile
# bump does (see main.yml's grafana_dockerfile path filter). Re-tagged and
# pushed to this repo's own GHCR namespace so config/deploy.grafana.yml can
# deploy it the same way api/web deploy their own images, reusing the
# existing KAMAL_REGISTRY_USERNAME/PASSWORD GHCR credentials.
#
# Bumping Grafana's version means bumping the tag below.
FROM grafana/grafana:13.2.1

# Grafana reads these paths on startup without any GF_PATHS_* override.
COPY infra/grafana/provisioning/ /etc/grafana/provisioning/
COPY infra/grafana/dashboards/ /var/lib/grafana/dashboards/
