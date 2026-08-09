# P5 Trace Propagation

- HTTP：W3C traceparent + X-Request-ID
- Task 入队保存 trace_parent, correlation_id, request_id
- Worker consumer span link 父 trace
- 非法 traceparent 创建新 trace；失败不阻塞任务
