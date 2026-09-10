hooks/test:
	./scripts/test_hooks.sh

# Validate the provisioned Grafana dashboards (issue #1527). Nothing else in
# the pipeline reads infra/grafana/dashboards/*.json — CI runs this via
# build-grafana.yml before the image build.
lint/grafana:
	python3 scripts/validate_grafana_dashboards.py
