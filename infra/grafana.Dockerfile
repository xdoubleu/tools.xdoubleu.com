# Trivial wrapper around the stock grafana/grafana-oss image (issue #1509).
# Exists purely so build-grafana.yml can stamp the `service` docker label
# Kamal's `validate_image` step requires on every deployed image (see
# build-api.yml's identical comment) — the upstream image obviously carries
# no such label. No other change: same entrypoint, same config, just
# re-tagged and pushed to this repo's own GHCR namespace so
# config/deploy.grafana.yml can deploy it the same way api/web deploy their
# own images, reusing the existing KAMAL_REGISTRY_USERNAME/PASSWORD GHCR
# credentials instead of a separate Docker Hub account (#1505's
# DOCKERHUB_USERNAME/TOKEN, reverted here as unnecessary).
#
# Bumping Grafana's version means bumping the tag below.
FROM grafana/grafana:12.4.10
