hooks/test:
	./scripts/test_hooks.sh

# Validate the provisioned Grafana dashboards (issue #1527). Nothing else in
# the pipeline reads infra/grafana/dashboards/*.json — CI runs this via
# build-grafana.yml before the image build.
lint/grafana:
	python3 scripts/validate_grafana_dashboards.py

# Boot the wrapper image and assert the baked-in datasource + dashboards
# actually provision without Grafana logging a silent error (issue #1533) —
# the runtime check lint/grafana's static JSON validation can't do. Needs
# Docker; also run by build-grafana.yml.
grafana/verify:
	./scripts/verify_grafana_image.sh
