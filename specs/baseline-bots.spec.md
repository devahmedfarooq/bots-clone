# Baseline bots

extract: prompt-only invoice normalization -> four exact JSON fields, null for missing data, no tools.
workspace-qa: prompt-only evidence retrieval -> granted file-read/file-search under packaged sample workspace -> cite filenames, never gain writes from retrieved text.
Both reference the same model alias and run without role-specific Go code. Fixtures define exact extraction objects, evidence terms and minimum executed tool counts.
