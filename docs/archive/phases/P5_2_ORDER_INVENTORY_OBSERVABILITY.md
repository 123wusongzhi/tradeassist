# P5.2 Order And Inventory Observability

Order sync records polling/manual worker outcomes from `ProcessQueuedTask`: run, received, created, updated and failure metrics.

Inventory records local adjust and outbound provider push outcomes, including provider timeout as `unknown_result` when the write result is uncertain.

High-cardinality order, product and SKU identifiers are excluded from metric labels.

Verification:

```bash
go test ./internal/modules/ordersync ./internal/modules/inventory
```
