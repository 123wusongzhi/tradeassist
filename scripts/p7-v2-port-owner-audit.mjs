import { captureApiProcessIdentity } from './p7-v2-process-identity.mjs';
import { readJSON, resolveP7V2PortConfig, run, runWSL, wslProjectRoot, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const portConfig = resolveP7V2PortConfig();
const port = portConfig.port;
const pidFile = `${wslProjectRoot()}/artifacts/p7-v2/server.pid`;

function lines(value) {
  return String(value || '').split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
}

function wslListeners(targetPort) {
  const result = runWSL(`ss -ltnp 'sport = :${targetPort}' 2>/dev/null || true`, { timeout: 15000 });
  const owners = [...String(result.stdout || '').matchAll(/pid=(\d+)/g)].map((match) => match[1]);
  return { status: result.status === 0 ? 'available' : 'probe_unavailable', raw: lines(result.stdout), pids: [...new Set(owners)] };
}

function windowsListeners(targetPort) {
  const command = [
    `$c=Get-NetTCPConnection -LocalPort ${targetPort} -State Listen -ErrorAction SilentlyContinue`,
    '$c|ForEach-Object {$p=$_.OwningProcess;$x=Get-CimInstance Win32_Process -Filter ("ProcessId="+$p);[PSCustomObject]@{localAddress=$_.LocalAddress;localPort=$_.LocalPort;state=$_.State;pid=$p;name=$x.Name;executablePath=$x.ExecutablePath;commandLine=$x.CommandLine;creationDate=$x.CreationDate}}|ConvertTo-Json -Compress',
  ].join(';');
  const result = run('powershell.exe', ['-NoProfile', '-Command', command], { timeout: 30000 });
  try {
    const parsed = JSON.parse(String(result.stdout || '').trim() || '[]');
    return { status: result.status === 0 ? 'available' : 'probe_unavailable', listeners: Array.isArray(parsed) ? parsed : [parsed] };
  } catch {
    return { status: 'probe_unavailable', listeners: [] };
  }
}

const wsl = wslListeners(port);
const windows = windowsListeners(port);
const pid = wsl.pids.length === 1 ? wsl.pids[0] : '';
const identity = pid ? captureApiProcessIdentity({ pid, port }) : { present: false };
const pidFileResult = runWSL(`cat ${JSON.stringify(pidFile)} 2>/dev/null || true`, { timeout: 10000 });
const pidFileValue = String(pidFileResult.stdout || '').trim();
const projectRoot = wslProjectRoot();
const binary = `${projectRoot}/artifacts/p7-v2/server`;
const strongEvidence = [
  identity.workingDirectory === projectRoot,
  identity.executableRealPath === binary || identity.executablePath === binary,
  pid && pid === pidFileValue,
  Boolean(identity.instanceNonce),
].filter(Boolean).length;
const belongsToTradeMind = strongEvidence >= 2;
const hasListener = wsl.pids.length > 0 || windows.listeners.length > 0;
const classification = !hasListener
  ? 'false_positive'
  : belongsToTradeMind
    ? 'stale_trademind_process'
    : wsl.pids.length && windows.listeners.length
      ? 'wsl_forwarding_process'
      : 'unknown';
const portproxy = run('netsh', ['interface', 'portproxy', 'show', 'all'], { timeout: 15000 });
const docker = run('docker', ['ps', '--format', '{{.ID}}\t{{.Names}}\t{{.Ports}}'], { timeout: 15000 });
const report = {
  phase: 'P7-V2-R3B-PORT-R2',
  status: wsl.status === 'probe_unavailable' || windows.status === 'probe_unavailable' ? 'incomplete' : 'passed',
  selectedHost: portConfig.host,
  selectedPort: port,
  classification,
  wsl: { ...wsl, identity: identity.present ? identity : null, pidFile: pidFileValue },
  windows: windows,
  portproxy: lines(portproxy.stdout),
  docker: docker.status === 0 ? lines(docker.stdout) : [],
  belongsToTradeMind,
  strongEvidenceCount: strongEvidence,
  unknownProcessTerminated: false,
  unrelatedServiceAffected: false,
  generatedAt: new Date().toISOString(),
};
writeJSON('docs/p7-v2-r3b-port-owner-audit.json', report);
writeMarkdown(
  'docs/P7_V2_R3B_PORT_OWNER_AUDIT.md',
  `# P7-V2-R3B-PORT-R2 Port Owner Audit\n\nStatus: **${report.status}**\n\n- Port: \`${port}\`\n- Classification: \`${classification}\`\n- TradeMind evidence count: ${strongEvidence}\n- Unknown process terminated: false\n- Unrelated service affected: false\n`,
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
