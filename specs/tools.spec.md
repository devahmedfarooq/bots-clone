# Tools and filesystem permissions

```text
call(grants, name, arguments):
  require_registered(name); require_granted(grants, name)
  validate_typed_arguments(arguments)
  return bounded_result(execute_with_timeout(arguments)) OR structured_error
file_operation(root_index, relative_path):
  require_granted_root_index(); require_local_path_without_volume_or_ADS()
  operate_through_pinned_os_root_handle()
```

Plugin interface: Definition(name, description, JSON schema), Validate(raw arguments), Execute(context, arguments).
Register unique valid names -> expose granted schemas only -> check existence AND grant independently -> strict argument validation -> bounded timeout/output -> structured result/error. Custom trusted in-process implementations must obey context cancellation; external executable plugins are Phase 3.
File tools pin os.Root handles for declared context directories. Accept root index + local relative path; reject absolute paths, traversal, volume/ADS paths. os.Root performs handle-based symlink/junction confinement, including races.
Read regular files <=64 KiB. Search literal text: <=2000 entries, <=64 KiB per file, <=16 KiB result; skip symlink entries and Git/cache dirs. Write creates exclusively <=64 KiB, never overwrites, no implicit parent directories. All operations need explicit tool grants.
