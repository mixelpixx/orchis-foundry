-- 0013_scanner_budget: per-user LLM context controls.
--   context_budget    — approximate input token cap fed to the model (guards
--                        local VRAM / cloud cost). Bytes ≈ tokens * 4.
--   max_output_tokens — generation cap passed as the provider's max_tokens.
--   prune_globs        — newline/comma-separated extra path globs to strip from
--                        LLM context (in addition to the built-in defaults).
ALTER TABLE scanner_settings ADD COLUMN context_budget INTEGER NOT NULL DEFAULT 8000;
ALTER TABLE scanner_settings ADD COLUMN max_output_tokens INTEGER NOT NULL DEFAULT 2048;
ALTER TABLE scanner_settings ADD COLUMN prune_globs TEXT NOT NULL DEFAULT '';
