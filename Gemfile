# Pins the Kamal gem (issue #1036) — Kamal has no Go/Node distribution, it
# ships as a Ruby gem. Used locally (infra/main.tf's null_resource.kamal_deploy
# also runs a bare `gem install kamal`, unrelated to this Gemfile) and in CI
# (.github/workflows/main.yml's deploy-kamal job, via `bundle exec kamal`).
source "https://rubygems.org"

gem "kamal"
