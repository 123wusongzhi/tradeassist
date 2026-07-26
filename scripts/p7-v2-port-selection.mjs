import { captureApiProcessIdentity } from './p7-v2-process-identity.mjs';
import { readJSON, resolveP7V2PortConfig, run, runWSL, stopP7V2Server, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

function wslFree(port) {
  const res = runWSL(`ss -ltn 'sport = :${port}' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? "busy" : "free"}'`, { timeout: 10000 });
  return res.status === 0 && String(res.stdout || '').trim() === 'free';
}

function windowsFree(port) {
  const res = run('powershell.exe', ['-NoProfile', '-Command', `(Get-NetTCPConnection -LocalPort ${port} -State Listen -ErrorAction SilentlyContinue|Measure-Object).Count`], { timeout: 15000 });
  return res.status === 0 && String(res.stdout || '').trim() === '0';
}

const configured = resolveP7V2PortConfig();
const audit = readJSON('docs/p7-v2-r3b-port-owner-audit.json') || {};
let selectedPort = configured.port;
let action = 'safe_release';
let released = false;
if (audit.classification === 'stale_trademind_process' && audit.belongsToTradeMind === true) {
  const identity = captureApiProcessIdentity({ pid: audit.wsl?.identity?.pid || '', port: configured.port });
  const stopped = stopP7V2Server({ expectedIdentity: identity, portConfig: configured });
  released = stopped.portReleased === true;
} else if (audit.classification !== 'false_positive') {
  action = 'port_migration';
  selectedPort = [18080, 28080, 38080].find((candidate) => wslFree(candidate) && windowsFree(candidate)) || 0;
}
const selectedAvailableInWSL = selectedPort > 0 && wslFree(selectedPort);
const selectedAvailableInWindows = selectedPort > 0 && windowsFree(selectedPort);
const report = {
  phase: 'P7-V2-R3B-PORT-R2',
  status: selectedPort > 0 && selectedAvailableInWSL && selectedAvailableInWindows && (action === 'port_migration' || released || audit.classification === 'false_positive') ? 'passed' : 'incomplete',
  originalPort: configured.port,
  originalPortClassification: audit.classification || 'unknown',
  originalPortOwner: {
    side: audit.wsl?.pids?.length && audit.windows?.listeners?.length ? 'both' : audit.wsl?.pids?.length ? 'wsl' : audit.windows?.listeners?.length ? 'windows' : 'none',
    pid: audit.wsl?.identity?.pid || null,
    executablePath: audit.wsl?.identity?.executablePath || '',
    workingDirectory: audit.wsl?.identity?.workingDirectory || '',
    commandLine: audit.wsl?.identity?.commandLine || '',
    belongsToTradeMind: audit.belongsToTradeMind === true,
  },
  action,
  selectedHost: configured.host,
  selectedPort,
  unknownProcessTerminated: false,
  unrelatedServiceAffected: false,
  safeReleasePortReleased: released,
  selectedPortAvailableInWSL: selectedAvailableInWSL,
  selectedPortAvailableInWindows: selectedAvailableInWindows,
  generatedAt: new Date().toISOString(),
};
writeJSON('docs/p7-v2-r3b-port-selection-report.json', report);
writeMarkdown('docs/P7_V2_R3B_PORT_SELECTION_REPORT.md', `# P7-V2-R3B-PORT-R2 Port Selection\n\nStatus: **${report.status}**\n\n- Original port: ${report.originalPort}\n- Classification: \`${report.originalPortClassification}\`\n- Action: \`${action}\`\n- Selected endpoint: \`${configured.host}:${selectedPort || 'unavailable'}\`\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
