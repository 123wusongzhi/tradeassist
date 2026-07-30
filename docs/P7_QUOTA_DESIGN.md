# P7 Quota Design

P7 creates `quota_policies` as the foundation table.

Initial quota scopes:

- tenant
- shop
- user
- system

Initial quota dimensions:

- active tasks
- AI text/image jobs
- exports
- webhook backlog
- provider calls
- upload bytes
- storage bytes

Runtime enforcement and management UI are pending.
