# P7-V2 Load Host Guard Report

Status: passed

- [passed] localhost-allowed: http://localhost:18080
- [passed] 127-allowed: http://127.0.0.1:18080
- [passed] wsl-nonloopback-rejected: http://172.22.144.1:18080
- [passed] production-domain-rejected: https://api.zhihengxiangyu.com
- [passed] public-ip-rejected: http://8.8.8.8:18080
- [passed] empty-host-rejected: <empty>

| Field | Value |
| --- | --- |
| Port 18080 available | true |
| Listener 18080 count | 0 |
| Unknown DBs | 0 |
| Current formal residual DBs | 0 |
| Unknown processes | 0 |
| Bind probe passed | true |
