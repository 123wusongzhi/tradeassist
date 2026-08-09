# P5 Log Redaction

复用 safefields + logging.SanitizeLogFields。Panic/Provider/Webhook 错误均走脱敏。测试注入 TEST_*_UNIQUE 断言不出现在日志/Trace/Alert。
