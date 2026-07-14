import { probePerformanceEndpoints, resolvePerformanceAuthStatus, resolvePerformanceAuthToken } from './p7-v2-lib.mjs';

const token = resolvePerformanceAuthToken();
console.log(JSON.stringify({ authStatus: resolvePerformanceAuthStatus(), tokenAvailable: Boolean(token), probe: probePerformanceEndpoints() }, null, 2));
