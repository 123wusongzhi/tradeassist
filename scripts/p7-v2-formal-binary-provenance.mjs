import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  buildBinaryProvenanceReceipt,
  BINARY_PROVENANCE_MANIFEST_PATH,
  freezeBinaryProvenance,
  FORMAL_BINARY_PROVENANCE_VERSION,
  readJSON,
  resolveRuntime,
  verifyBinaryReceipt,
  writeJSON,
} from './p7-v2-formal-binary-provenance-lib.mjs';

function valueOf(args, name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function printAndExit(report) {
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

export function runBinaryProvenanceCli(argv = process.argv.slice(2)) {
  argv = argv.filter((arg) => arg !== '--');
  const [command = 'resolve'] = argv;
  const role = valueOf(argv, '--role');
  const runtimeWorktree = valueOf(argv, '--runtime-worktree') || process.cwd();
  const runtimeCommit = valueOf(argv, '--runtime-commit');
  try {
    if (command === 'resolve') {
      return {
        phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
        status: 'passed',
        command,
        ...resolveRuntime({ role, runtimeWorktree, runtimeCommit }),
      };
    }
    if (command === 'build') {
      return {
        phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
        status: 'passed',
        command,
        ...buildBinaryProvenanceReceipt({ role, runtimeWorktree, runtimeCommit }),
      };
    }
    if (command === 'verify') {
      const receiptPath = valueOf(argv, '--receipt') || valueOf(argv, '--provenance');
      const verification = verifyBinaryReceipt(receiptPath || readJSON(BINARY_PROVENANCE_MANIFEST_PATH)?.binaryProvenance?.[role], { role, runtimeCommit });
      return {
        phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
        status: verification.status,
        command,
        formalBinaryProvenanceVersion: FORMAL_BINARY_PROVENANCE_VERSION,
        binaryVerified: verification.valid,
        issues: verification.issues,
        receiptPath,
        binaryPath: verification.receipt?.binaryPath || '',
        binarySha256: verification.receipt?.binarySha256 || '',
        runtimeCommit: verification.receipt?.runtimeCommit || '',
      };
    }
    if (command === 'freeze') {
      const baselineReceiptPath = valueOf(argv, '--baseline-receipt');
      const currentReceiptPath = valueOf(argv, '--current-receipt');
      const outputPath = valueOf(argv, '--output') || BINARY_PROVENANCE_MANIFEST_PATH;
      return freezeBinaryProvenance({ baselineReceiptPath, currentReceiptPath, outputPath });
    }
    return {
      phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
      status: 'failed',
      command,
      classification: 'unsupported_binary_provenance_command',
      supportedCommands: ['resolve', 'build', 'verify', 'freeze'],
    };
  } catch (error) {
    const report = {
      phase: 'P7-V2-R3B-FORMAL-BINARY-PROVENANCE-V2',
      status: 'failed',
      command,
      role,
      runtimeWorktree,
      runtimeCommit,
      error: error.message,
    };
    const outputPath = valueOf(argv, '--output');
    if (outputPath) writeJSON(outputPath, report);
    return report;
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  printAndExit(runBinaryProvenanceCli());
}
