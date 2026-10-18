# Retention

CLI cleanup requires cutoff timestamp -> acquire same process lock as service/inference -> inspect checkpoints -> protect all workflow-referenced runs + requested keep IDs + running records.
Store retention planning and direct deletion also protect dispatch-key job runs, preserving autonomous task evidence and restart deduplication.
Plan old terminal runs -> JSON dry run by default -> explicit -apply deletes messages/events/run rows and corresponding terminal job checkpoints transactionally -> remove orphan sessions.
All model/adapter/package artifacts preserved until Phase 2 reference-aware artifact storage exists. Never infer unused weights from filenames or delete arbitrary paths.
