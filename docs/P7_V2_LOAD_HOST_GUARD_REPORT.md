# P7-V2 Load Host Guard Report

Status: passed

- [passed] localhost-allowed: http://localhost:8080
- [passed] 127-allowed: http://127.0.0.1:8080
- [passed] wsl-controlled-allowed: http://172.22.144.1:8080
- [passed] production-domain-rejected: https://api.zhihengxiangyu.com
- [passed] public-ip-rejected: http://8.8.8.8:8080
- [passed] empty-host-rejected: <empty>
