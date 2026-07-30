import { describe, expect, it } from 'vitest';
import {
  classifyOpenCliCommandError,
  collectViaOpenCli,
  parseOpenCliDaemonStatusOutput,
  probeOpenCliStatus,
  type OpenCliRunner,
} from './opencli-driver.js';

describe('OpenCLI driver', () => {
  it('parses passive daemon readiness without exposing raw output', () => {
    const status = parseOpenCliDaemonStatusOutput(`
      Daemon: running (PID 123)
      Version: v1.8.6
      Extension: connected (v1.0.22)
      Profiles: xynwjrgw v1.0.22
    `);
    expect(status).toEqual({
      ready: true,
      binaryAvailable: true,
      daemonRunning: true,
      extensionConnected: true,
      profileAvailable: true,
    });
  });

  it('probes status passively without running doctor or leasing a browser window', async () => {
    let argv: string[] = [];
    const runner: OpenCliRunner = async (args) => {
      argv = args;
      return {
        stderr: '',
        stdout: `
          Daemon: running (PID 123)
          Extension: connected (v1.0.22)
          Profiles: xynwjrgw v1.0.22
        `,
      };
    };

    const status = await probeOpenCliStatus(runner);

    expect(argv).toEqual(['daemon', 'status']);
    expect(argv).not.toContain('doctor');
    expect(status.ready).toBe(true);
  });

  it('does not mark an unselected multi-profile daemon as ready', () => {
    const status = parseOpenCliDaemonStatusOutput(`
      Daemon: running (PID 123)
      Extension: 2 profiles connected, none selected
      Profiles: profile-a v1.0.22, profile-b v1.0.22
    `);

    expect(status.daemonRunning).toBe(true);
    expect(status.profileAvailable).toBe(true);
    expect(status.extensionConnected).toBe(false);
    expect(status.ready).toBe(false);
  });

  it('normalizes product JSON and passes URL as one argv item', async () => {
    let argv: string[] = [];
    const runner: OpenCliRunner = async (args) => {
      argv = args;
      return {
        stderr: '',
        stdout: JSON.stringify([
          {
            title: 'Example product',
            currency: 'CNY',
            mainImages: ['https://img.example/main.jpg'],
            skus: [{ skuCode: 'SKU-1', price: 12.5 }],
          },
        ]),
      };
    };
    const url = 'https://item.taobao.com/item.htm?id=1&spm=test';
    const product = await collectViaOpenCli('taobao_tmall', url, { skuClickMaxCount: 8 }, runner);

    expect(argv).toContain(url);
    expect(argv).toContain('8');
    expect(product.title).toBe('Example product');
    expect(product.raw.engine).toBe('opencli');
    expect(product.skus[0]?.id).toBe('SKU-1');
  });

  it('does not treat OpenCLI EMPTY_RESULT as proof that a product was removed', () => {
    const error = classifyOpenCliCommandError(
      { code: 66, message: 'Command failed' },
      `ok: false
error:
  code: EMPTY_RESULT
  message: tmall product returned no data
  help: 商品不存在或已下架`,
      '',
    );

    expect(error.code).toBe('PARSE_FAILED');
    expect(error.message).toContain('may require login or verification');
  });

  it('keeps only an explicit ITEM_NOT_FOUND code as a definitive missing item', () => {
    const error = classifyOpenCliCommandError(
      { code: 66, message: 'Command failed' },
      `ok: false
error:
  code: ITEM_NOT_FOUND
  message: product is no longer available`,
      '',
    );

    expect(error.code).toBe('ITEM_NOT_FOUND');
  });

  it('preserves specific parse failures carried inside EMPTY_RESULT hints', () => {
    const error = classifyOpenCliCommandError(
      { code: 66, message: 'Command failed' },
      `ok: false
error:
  code: EMPTY_RESULT
  help: 未能提取到商品主图（MAIN_IMAGES_EMPTY）`,
      '',
    );

    expect(error.code).toBe('MAIN_IMAGES_EMPTY');
  });
});
