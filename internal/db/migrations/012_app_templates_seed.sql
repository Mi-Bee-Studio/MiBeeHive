-- 012_app_templates_seed.sql
-- Seed default application templates.

INSERT INTO app_templates (name, description, image, ports, env, category) SELECT 'nginx', 'Nginx Web Server', 'nginx:alpine', '[{"host_port":80,"container_port":80,"protocol":"tcp"}]', '{}', 'web' WHERE NOT EXISTS (SELECT 1 FROM app_templates WHERE name = 'nginx');
INSERT INTO app_templates (name, description, image, ports, env, category) SELECT 'redis', 'Redis Key-Value Store', 'redis:alpine', '[{"host_port":6379,"container_port":6379,"protocol":"tcp"}]', '{}', 'database' WHERE NOT EXISTS (SELECT 1 FROM app_templates WHERE name = 'redis');
INSERT INTO app_templates (name, description, image, ports, env, category) SELECT 'postgres', 'PostgreSQL Database', 'postgres:alpine', '[{"host_port":5432,"container_port":5432,"protocol":"tcp"}]', '{"POSTGRES_PASSWORD":"changeme"}', 'database' WHERE NOT EXISTS (SELECT 1 FROM app_templates WHERE name = 'postgres');
