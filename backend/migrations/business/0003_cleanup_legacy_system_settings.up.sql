DELETE FROM system_settings
WHERE setting_key LIKE 'legacy.%'
   OR setting_key LIKE 'resource.default_permission.%'
   OR setting_key IN (
       'assessment.org_scopes',
       'assessment.permission_bindings_legacy_fallback'
   );
