# P5 Alerting Design

模型：alert_events, alert_rules, alert_silences。去重 fingerprint + cooldown + recovery。Channel adapter：internal（默认）, email/webhook deferred。
