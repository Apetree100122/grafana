UPDATE "resource_version"
SET
    "resource_version" = CAST((julianday('now') - 2440587.5) * 86400000000.0 AS INTEGER)
WHERE 1 = 1
    AND "group"    = ''
    AND "resource" = ''
;
