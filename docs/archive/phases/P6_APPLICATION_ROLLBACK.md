# P6 Application Rollback

Application rollback may:

- switch current link back to previous version
- restore previous Nginx config
- stop a failed candidate
- record rollback audit rows

Application rollback must not:

- restore a database
- run destructive down migrations
- delete new data
- delete backup records
- clear task tables

