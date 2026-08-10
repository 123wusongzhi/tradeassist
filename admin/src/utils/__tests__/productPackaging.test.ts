import { describe, expect, it } from 'vitest';
import { collectedPackagingRowsFromRaw } from '../productPackaging';

describe('collected packaging raw data', () => {
  it('keeps explicit zeroes and missing measurements distinct', () => {
    expect(
      collectedPackagingRowsFromRaw({
        packaging: {
          rows: [
            {
              specification: '22#橡胶塞',
              lengthCm: 1,
              widthCm: 1,
              heightCm: 1,
              volumeCm3: 1,
              weightG: 2000,
            },
            {
              specification: '双孔8#橡胶塞',
              lengthCm: null,
              widthCm: null,
              heightCm: null,
              volumeCm3: null,
              weightG: 100,
            },
            {
              specification: '真实零值',
              lengthCm: 0,
              widthCm: 0,
              heightCm: 0,
              volumeCm3: 0,
              weightG: 0,
            },
          ],
        },
      }),
    ).toHaveLength(3);
  });

  it('rejects malformed rows instead of coercing or inferring values', () => {
    expect(
      collectedPackagingRowsFromRaw({
        packaging: {
          rows: [
            { specification: '字符串重量', lengthCm: 1, widthCm: 1, heightCm: 1, volumeCm3: 1, weightG: '100' },
            { specification: '负数', lengthCm: -1, widthCm: 1, heightCm: 1, volumeCm3: 1, weightG: 100 },
            { specification: '', lengthCm: 1, widthCm: 1, heightCm: 1, volumeCm3: 1, weightG: 100 },
            { specification: '超长'.repeat(101), lengthCm: 1, widthCm: 1, heightCm: 1, volumeCm3: 1, weightG: 100 },
          ],
        },
      }),
    ).toEqual([]);
  });
});
