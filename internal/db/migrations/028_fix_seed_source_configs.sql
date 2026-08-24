-- 028_fix_seed_source_configs.sql
-- Repair broken seed project configs for existing deployments (issue #60).
--
-- 1. HashiCorp projects (consul/packer/vagrant/nomad) were seeded without the
--    product name in github_owner, so fetches hit /v1/releases/ (empty product)
--    and always failed with 403. The project name IS the product name.
-- 2. The vmagent project pointed at VictoriaMetrics/vmagent, a repository that
--    does not exist (vmagent ships inside the vmutils-* archives of the main
--    VictoriaMetrics repo). Point it at the main repo and scope it to the
--    vmutils-* archives via filter_patterns.
--
-- json_valid/json_set guards keep rows with unexpected config shapes intact.

UPDATE projects
SET config = json_set(config, '$.github_owner', name)
WHERE source_type = 'hashicorp'
  AND json_valid(config)
  AND COALESCE(json_extract(config, '$.github_owner'), '') = '';

UPDATE projects
SET config = json_set(config,
        '$.github_repo', 'VictoriaMetrics',
        '$.filter_patterns', json('["vmutils-*"]'))
WHERE name = 'vmagent'
  AND source_type = 'github'
  AND json_valid(config)
  AND COALESCE(json_extract(config, '$.github_repo'), '') = 'vmagent';
