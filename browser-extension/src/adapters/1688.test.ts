import { describe, expect, it } from 'vitest';
import type { ProductSku } from '../types.js';
import {
  extract1688OfferPriceFromUnknown,
  extract1688SkuPriceFromBucket,
  collect1688Page,
  extract1688TitleFromUnknown,
  extract1688DescriptionImagesFromUnknown,
  extract1688MainDescriptionFromUnknown,
  extract1688PackagingFromUnknown,
  extract1688ProductAttributesFromUnknown,
  extract1688ProductDimensionsFromUnknown,
  extract1688SkuImageRowsFromUnknown,
  extract1688SkuPropertyRowsFromUnknown,
  extract1688StructuredSkuImageRowsFromUnknown,
  merge1688ProductDimensionsIntoSkus,
  merge1688SkuImagesIntoSkus,
  merge1688SkuPropertiesIntoSkus,
  extractMinOrderFromUnknown,
  extractPriceTiersFromUnknown,
  extract1688OfferId,
  isSupported1688URL,
  isValid1688SkuDimensionValue,
  parse1688SkuComboKey,
  parse1688Price,
  parse1688Quantity,
} from './1688.js';

describe('1688 pure helpers', () => {
  it('accepts offer detail URLs only', () => {
    expect(isSupported1688URL('https://detail.1688.com/offer/1054514049952.html')).toBe(true);
    expect(isSupported1688URL('https://detail.1688.com/offer/1054514049952.html?offerId=1054514049952')).toBe(
      true,
    );
    expect(isSupported1688URL('https://m.1688.com/offer/1.html')).toBe(true);
    expect(isSupported1688URL('https://www.1688.com/')).toBe(false);
    expect(isSupported1688URL('http://detail.1688.com/offer/1.html')).toBe(false);
    expect(isSupported1688URL('https://www.1688.com/offer/1.html')).toBe(false);
    expect(isSupported1688URL('https://detail.1688.com/offer/')).toBe(false);
    expect(isSupported1688URL('https://detail.1688.com/offer/not-a-number.html')).toBe(false);
    expect(isSupported1688URL('https://detail.1688.com/offer.html')).toBe(false);
    expect(isSupported1688URL('https://detail.tmall.com/item.htm?id=1')).toBe(false);
  });

  it('extracts offer id from path and query', () => {
    expect(extract1688OfferId('https://detail.1688.com/offer/1054514049952.html')).toBe('1054514049952');
    expect(extract1688OfferId('https://detail.1688.com/?offerId=1054514049952')).toBe('1054514049952');
    expect(extract1688OfferId('https://detail.1688.com/?offerId=not-a-number')).toBeUndefined();
  });

  it('rejects structured title values instead of coercing them to object text', () => {
    expect(extract1688TitleFromUnknown([{ title: { text: '对象里的标题不应被隐式采用' } }])).toBeUndefined();
    expect(extract1688TitleFromUnknown([{ title: ['数组标题也不应被隐式采用'] }])).toBeUndefined();
  });

  it('finds a valid subject nested in page model data', () => {
    expect(
      extract1688TitleFromUnknown([
        {
          result: {
            data: {
              subject: '白色橡胶塞实验室密封塞',
            },
          },
        },
      ]),
    ).toBe('白色橡胶塞实验室密封塞');
  });

  it('falls back from invalid model titles to DOM, Open Graph, and document titles', () => {
    expect(
      extract1688TitleFromUnknown([{ title: { text: '不能隐式采用' } }], {
        domHeadings: ['', 'DOM 商品标题'],
        ogTitle: 'OG 商品标题',
        documentTitle: '页面商品标题 - 1688.com',
      }),
    ).toBe('DOM 商品标题');
    expect(
      extract1688TitleFromUnknown([], {
        domHeadings: [''],
        ogTitle: 'OG 商品标题',
        documentTitle: '页面商品标题 - 1688.com',
      }),
    ).toBe('OG 商品标题');
    expect(
      extract1688TitleFromUnknown([], {
        documentTitle: '页面商品标题 - 1688.com',
      }),
    ).toBe('页面商品标题');
  });

  it('extracts lazy, srcset, background, and structured detail images from offline fixtures', () => {
    const images = extract1688DescriptionImagesFromUnknown({
      baseUrl: 'https://detail.1688.com/offer/123.html',
      domCandidates: [
        { attributes: { 'data-src': '//cbu01.alicdn.com/img/ibank/detail-lazy.jpg' } },
        {
          attributes: {
            srcset:
              '//cbu01.alicdn.com/img/ibank/detail-small.jpg 320w, //cbu01.alicdn.com/img/ibank/detail-large.jpg 1200w',
          },
        },
        { backgroundImage: 'linear-gradient(#fff,#eee), url("//cbu01.alicdn.com/img/ibank/detail-bg.webp")' },
      ],
      structuredRoots: [
        {
          result: {
            detailContent: '<img src="https:\\/\\/cbu01.alicdn.com\\/img\\/ibank\\/detail-json.png">',
          },
        },
      ],
    });

    expect(images).toEqual([
      'https://cbu01.alicdn.com/img/ibank/detail-lazy.jpg',
      'https://cbu01.alicdn.com/img/ibank/detail-large.jpg',
      'https://cbu01.alicdn.com/img/ibank/detail-bg.webp',
      'https://cbu01.alicdn.com/img/ibank/detail-json.png',
    ]);
  });

  it('keeps main, SKU, service, icon, and tiny images out of description images', () => {
    const main = 'https://cbu01.alicdn.com/img/ibank/shared-main.jpg?x-oss-process=resize';
    const sku = 'https://cbu01.alicdn.com/img/ibank/shared-sku.jpg';
    const images = extract1688DescriptionImagesFromUnknown({
      mainImages: [main],
      skuImages: [sku],
      domCandidates: [
        { attributes: { src: main } },
        { attributes: { src: sku }, ancestorHint: 'sku-selector sale-prop' },
        { attributes: { src: 'https://assets.example.com/service-icon.png' } },
        {
          attributes: { src: 'https://cbu01.alicdn.com/img/ibank/tiny_32x32.png' },
          naturalWidth: 32,
          naturalHeight: 32,
        },
        { attributes: { src: 'https://cbu01.alicdn.com/img/ibank/real-detail.jpg' } },
      ],
      structuredRoots: [
        {
          detailContent: {
            imageUrl: main,
            skuImages: [sku],
            gallery: ['https://cbu01.alicdn.com/img/ibank/gallery-main.jpg'],
          },
        },
      ],
    });

    expect(images).toEqual(['https://cbu01.alicdn.com/img/ibank/real-detail.jpg']);
  });

  it('falls through placeholder src values to currentSrc and srcset candidates', () => {
    const images = extract1688DescriptionImagesFromUnknown({
      domCandidates: [
        {
          attributes: { src: 'https://assets.example.com/loading-placeholder.gif' },
          currentSrc: 'https://cbu01.alicdn.com/img/ibank/detail-current.jpg',
        },
        {
          attributes: {
            src: 'data:image/gif;base64,placeholder',
            srcset:
              'https://assets.example.com/loading.gif 320w, https://cbu01.alicdn.com/img/ibank/detail-srcset.webp 1200w',
          },
          currentSrc: 'https://assets.example.com/placeholder.gif',
        },
      ],
    });

    expect(images).toEqual([
      'https://cbu01.alicdn.com/img/ibank/detail-current.jpg',
      'https://cbu01.alicdn.com/img/ibank/detail-srcset.webp',
    ]);
  });

  it('bounds cyclic structured traversal without preventing a later root from being scanned', () => {
    const cyclic: Record<string, unknown> = {};
    cyclic.self = cyclic;
    cyclic.detailContent = Array.from({ length: 10_050 }, (_, index) => `non-image-${index}`);

    expect(
      extract1688DescriptionImagesFromUnknown({
        structuredRoots: [
          cyclic,
          { detailImages: ['https://cbu01.alicdn.com/img/ibank/detail-after-budget.jpg'] },
        ],
      }),
    ).toEqual(['https://cbu01.alicdn.com/img/ibank/detail-after-budget.jpg']);
  });

  it('prefers trustworthy detail DOM copy over structured and meta descriptions', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        productTitle: '白色橡胶塞商品标题',
        domCandidates: [
          {
            contextText: '商品详情',
            sourceHint: 'detail-content',
            html: '<section><h2>商品详情</h2><p>采用高弹橡胶材质，密封耐磨且不易变形。</p><p>适用于实验器皿和工业管口防护。</p></section>',
          },
        ],
        structuredRoots: [{ productDescription: '结构化描述不应覆盖可信详情正文。' }],
        metaDescription: 'Meta 描述不应覆盖可信详情正文。',
      }),
    ).toBe('采用高弹橡胶材质，密封耐磨且不易变形。\n适用于实验器皿和工业管口防护。');
  });

  it('falls back to an explicit structured description when detail DOM is unavailable', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        domCandidates: [{ contextText: '页面导航', text: '首页 商品分类 联系客服' }],
        structuredRoots: [
          {
            detailData: {
              productDescription: '<div>塞体弹性稳定，可重复使用并保持良好密封。</div>',
              sellingPoints: ['材料耐磨，安装后可减少管口碰伤。'],
            },
          },
        ],
        metaDescription: 'Meta 末级回退内容不应被采用。',
      }),
    ).toBe('塞体弹性稳定，可重复使用并保持良好密封。\n材料耐磨，安装后可减少管口碰伤。');
  });

  it('uses a meaningful meta description only as the final fallback', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        structuredRoots: [{ unrelated: { title: '普通对象标题', content: '非描述字段内容' } }],
        metaDescription: '支持多种口径选择，适用于教学实验与工业防护。',
      }),
    ).toBe('支持多种口径选择，适用于教学实验与工业防护。');
  });

  it('drops URL and image-path fragments while preserving real structured copy', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        structuredRoots: [
          {
            productDescription: [
              'https://cbu01.alicdn.com/img/ibank/detail.jpg',
              'data:image/gif;base64,placeholder',
              '/img/ibank/detail.webp',
              '真实商品正文仍应从混合描述数组中保留下来。',
            ],
          },
        ],
      }),
    ).toBe('真实商品正文仍应从混合描述数组中保留下来。');
  });

  it('cleans HTML, removes scripts and styles, and deduplicates repeated copy', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        domCandidates: [
          {
            sourceHint: 'offer-description',
            html: '<p>高弹耐磨，长期使用不易变形。</p><script>window.secret="脚本不得采集"</script><style>.x{content:"样式不得采集"}</style>&lt;script&gt;encodedSecret="转义脚本不得采集"&lt;/script&gt;<p>高弹耐磨，长期使用不易变形。</p><div>表面平整，安装拆卸方便。</div>',
          },
        ],
      }),
    ).toBe('高弹耐磨，长期使用不易变形。\n表面平整，安装拆卸方便。');
  });

  it('removes embedded specification and packaging tables from trusted detail HTML', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        domCandidates: [
          {
            contextText: '商品详情',
            html: [
              '<p>橡胶塞采用弹性材料制成，适合重复安装使用。</p>',
              '<table><tr><th>产品规格</th><th>产品尺寸</th></tr><tr><td>5#橡胶塞</td><td>29×22×28</td></tr></table>',
              '<table><tr><th>商品件重尺</th><th>重量(g)</th></tr><tr><td>5#橡胶塞</td><td>2000</td></tr></table>',
            ].join(''),
          },
        ],
      }),
    ).toBe('橡胶塞采用弹性材料制成，适合重复安装使用。');
  });

  it('keeps SKU, specification, packaging, price, and stock text out of descriptions', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        domCandidates: [
          { contextText: '产品规格', sourceHint: 'detail-content sku-table', text: '13×8×17 白色橡胶塞' },
          { contextText: '包装信息 商品件重尺', sourceHint: 'offer-description', text: '长1宽1高1重量2000克' },
          { contextText: '价格 库存', sourceHint: 'detail-content', text: '起批量2件 单价12元 库存200件' },
        ],
        structuredRoots: [
          { skuModel: { description: 'SKU 规格说明不得作为商品描述。' } },
          { productPackInfo: { detailContent: '包装重量尺寸不得作为商品描述。' } },
          { priceModel: { sellingPoints: ['价格阶梯内容不得作为商品描述。'] } },
          { stockModel: { productDescription: '库存数量不得作为商品描述。' } },
        ],
      }),
    ).toBeUndefined();
  });

  it('isolates cyclic and truly exhausted traversal budgets between structured roots', () => {
    const exhausted: Record<string, unknown> = {};
    exhausted.self = exhausted;
    exhausted.description = Array.from({ length: 30 }, (_, index) => ({ noise: index }));

    expect(
      extract1688MainDescriptionFromUnknown({
        nodeBudgetPerRoot: 8,
        structuredRoots: [
          exhausted,
          { productDescription: '后续独立数据根中的可信商品描述仍应正常采集。' },
        ],
      }),
    ).toBe('后续独立数据根中的可信商品描述仍应正常采集。');
  });

  it('caps collected description text at 5000 characters', () => {
    const description = extract1688MainDescriptionFromUnknown({
      domCandidates: [
        {
          contextText: '商品详情',
          text: '耐磨密封适用于多种工业场景。'.repeat(500),
        },
      ],
    });

    expect(description).toHaveLength(5000);
    expect(description?.startsWith('耐磨密封适用于多种工业场景。')).toBe(true);
  });

  it('returns undefined when every description candidate is empty or untrustworthy', () => {
    expect(
      extract1688MainDescriptionFromUnknown({
        productTitle: '白色橡胶塞商品标题',
        domCandidates: [
          { contextText: '页面导航', text: '首页 分类 登录' },
          { contextText: '商品详情', text: '白色橡胶塞商品标题' },
          { contextText: '卖点', text: '耐磨' },
        ],
        structuredRoots: [{ title: '白色橡胶塞商品标题' }, { skuModel: { description: '规格描述噪音' } }],
        metaDescription: '商品详情',
      }),
    ).toBeUndefined();
  });

  it('extracts 商品件重尺 rows with explicit units and preserves missing cells', () => {
    expect(
      extract1688PackagingFromUnknown([
        {
          contextText: '包装信息 商品件重尺',
          headers: ['产品规格', '长(cm)', '宽（cm）', '高 cm', '体积(cm³)', '重量(g)'],
          rows: [
            ['22#橡胶塞', '1', '1', '1', '1', '2,000'],
            ['双孔8#橡胶塞', '—', '-', '–', '--', '100'],
            ['真实零值', '0', '0', '0', '0', '0'],
          ],
        },
      ]),
    ).toEqual({
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
    });
  });

  it('rejects product dimension tables, unlabelled tables, and unit-bearing cell guesses', () => {
    expect(
      extract1688PackagingFromUnknown([
        {
          contextText: '产品规格',
          headers: ['产品规格', '产品尺寸', '品名'],
          rows: [['5#橡胶塞', '29*22*28', '5#橡胶塞']],
        },
        {
          contextText: '普通参数表',
          headers: ['产品规格', '长(cm)', '宽(cm)', '高(cm)', '体积(cm3)', '重量(g)'],
          rows: [['错误表', '1', '1', '1', '1', '100']],
        },
        {
          contextText: '包装信息 商品件重尺',
          headers: ['产品规格', '长(cm)', '宽(cm)', '高(cm)', '体积(cm3)', '重量(g)'],
          rows: [
            ['不得换算', '10mm', '1', '1', '1', '0.1kg'],
            ['超长'.repeat(101), '1', '1', '1', '1', '100'],
          ],
        },
      ]),
    ).toBeUndefined();
  });

  it('extracts the explicit 产品规格 / 产品尺寸 / 品名 table and normalizes only dimension separators', () => {
    expect(
      extract1688ProductDimensionsFromUnknown([
        {
          contextText: '产品规格',
          headers: ['产品规格', '产品尺寸', '品名'],
          rows: [
            ['000#橡胶塞', '13*8*17', '000#橡胶塞'],
            ['00#橡胶塞', '15 x 11 X 21 cm', '00#橡胶塞'],
            ['0#橡胶塞', '17×13×24（mm）', '0#橡胶塞'],
            ['1#橡胶塞', '19*14*26', ''],
          ],
        },
      ]),
    ).toEqual([
      { specification: '000#橡胶塞', productDimension: '13×8×17', productName: '000#橡胶塞' },
      { specification: '00#橡胶塞', productDimension: '15×11×21 cm', productName: '00#橡胶塞' },
      { specification: '0#橡胶塞', productDimension: '17×13×24（mm）', productName: '0#橡胶塞' },
      { specification: '1#橡胶塞', productDimension: '19×14×26', productName: '' },
    ]);
  });

  it('merges 产品尺寸 by one exact 产品规格 match, then uses 品名 only when 规格 has no match', () => {
    const skus: ProductSku[] = [
      { properties: { 规格: '000#橡胶塞' }, price: 1 },
      { properties: { 型号: '单孔7#橡胶塞' }, price: 2 },
      { properties: { 规格: '8#橡胶塞' }, price: 3 },
      { properties: { 规格: '1#橡胶塞' }, price: 4 },
    ];
    const merged = merge1688ProductDimensionsIntoSkus(skus, [
      { specification: '000#橡胶塞', productDimension: '13*8*17', productName: '000#橡胶塞' },
      { specification: '来源规格未进入SKU', productDimension: '37×28×30', productName: '单孔7#橡胶塞' },
      { specification: '9#橡胶塞', productDimension: '45×37×30', productName: '9#橡胶塞' },
      { specification: '1#橡胶塞', productDimension: '19×14×26', productName: '' },
    ]);

    expect(merged.map((sku) => sku.properties)).toEqual([
      { 规格: '000#橡胶塞', 产品尺寸: '13×8×17' },
      { 型号: '单孔7#橡胶塞', 产品尺寸: '37×28×30' },
      { 规格: '8#橡胶塞' },
      { 规格: '1#橡胶塞', 产品尺寸: '19×14×26' },
    ]);
    expect(skus[0]?.properties).toEqual({ 规格: '000#橡胶塞' });
  });

  it('rejects ambiguous, conflicting, malformed, packaging, and unmatched product-dimension rows', () => {
    expect(
      extract1688ProductDimensionsFromUnknown([
        {
          contextText: '包装信息 商品件重尺',
          headers: ['产品规格', '产品尺寸', '品名'],
          rows: [['5#橡胶塞', '29*22*28', '5#橡胶塞']],
        },
        {
          contextText: '产品规格',
          headers: ['产品规格', '长(cm)', '宽(cm)', '高(cm)', '品名'],
          rows: [['5#橡胶塞', '29', '22', '28', '5#橡胶塞']],
        },
        {
          contextText: '产品规格',
          headers: ['产品规格', '产品尺寸', '品名'],
          rows: [
            ['5#橡胶塞', '尺寸见详情', '5#橡胶塞'],
            ['6#橡胶塞', '33**25*28', '6#橡胶塞'],
          ],
        },
      ]),
    ).toEqual([]);

    const ambiguous = merge1688ProductDimensionsIntoSkus(
      [
        { properties: { 规格: '5#橡胶塞', 颜色: '白色' } },
        { properties: { 规格: '5#橡胶塞', 颜色: '黑色' } },
        { properties: { 规格: '6#橡胶塞' } },
        { properties: { 规格: '7#橡胶塞', 产品尺寸: '来源已有尺寸' } },
      ],
      [
        { specification: '5#橡胶塞', productDimension: '29×22×28', productName: '5#橡胶塞' },
        { specification: '6#橡胶塞', productDimension: '33×25×28', productName: '6#橡胶塞' },
        { specification: '6#橡胶塞', productDimension: '34×26×29', productName: '6#橡胶塞' },
        { specification: '7#橡胶塞', productDimension: '37×28×30', productName: '7#橡胶塞' },
        { specification: '未匹配', productDimension: '42×32×30', productName: '仍未匹配' },
      ],
    );
    expect(ambiguous.map((sku) => sku.properties)).toEqual([
      { 规格: '5#橡胶塞', 颜色: '白色' },
      { 规格: '5#橡胶塞', 颜色: '黑色' },
      { 规格: '6#橡胶塞' },
      { 规格: '7#橡胶塞', 产品尺寸: '来源已有尺寸' },
    ]);
  });

  it('extracts row thumbnails from the explicit SKU purchase table shown by 1688', () => {
    expect(
      extract1688SkuImageRowsFromUnknown([
        {
          headers: ['产品规格', '螺纹公称(M)', '公称长度(mm)', '价格 | 库存 (件)', '进货数量'],
          rows: [
            {
              cells: ['方8*40*100含挡板', '8', '100', '￥1.7 / 3990', '0'],
              image: '//cbu01.alicdn.com/img/ibank/O1CN01sku-100.jpg',
            },
            {
              cells: ['方8*40*120含挡板', '8', '120', '￥1.8 / 4815', '0'],
              image: 'data:image/gif;base64,placeholder',
              imageCandidates: ['//cbu01.alicdn.com/img/ibank/O1CN01sku-120.jpg'],
            },
          ],
        },
        {
          headers: ['产品规格', '产品尺寸', '品名'],
          rows: [
            {
              cells: ['不应从普通规格表采图', '10×20×30', '普通规格'],
              image: 'https://cbu01.alicdn.com/img/ibank/not-a-sku-row.jpg',
            },
          ],
        },
      ]),
    ).toEqual([
      {
        specification: '方8*40*100含挡板',
        image: 'https://cbu01.alicdn.com/img/ibank/O1CN01sku-100.jpg',
      },
      {
        specification: '方8*40*120含挡板',
        image: 'https://cbu01.alicdn.com/img/ibank/O1CN01sku-120.jpg',
      },
    ]);
  });

  it('extracts full structured skuProps images across supported metadata keys and caps output at 200', () => {
    const values = Array.from({ length: 205 }, (_, index) => ({
      name: `方8*40*${100 + index}含挡板`,
      ...(index % 3 === 0
        ? { imageUrl: `//cbu01.alicdn.com/img/ibank/structured-${index}.jpg` }
        : index % 3 === 1
          ? { picUrl: `https://cbu01.alicdn.com/img/ibank/structured-${index}.jpg` }
          : { image: { url: `https://cbu01.alicdn.com/img/ibank/structured-${index}.jpg` } }),
    }));
    const rows = extract1688StructuredSkuImageRowsFromUnknown([
      { result: { skuModel: { skuProps: [{ name: '规格', values }] } } },
    ]);

    expect(rows).toHaveLength(200);
    expect(rows[0]).toEqual({
      specification: '方8*40*100含挡板',
      image: 'https://cbu01.alicdn.com/img/ibank/structured-0.jpg',
    });
    expect(rows[199]?.specification).toBe('方8*40*299含挡板');
    expect(extract1688StructuredSkuImageRowsFromUnknown([{ skuProps: [{ values }] }], Number.NaN)).toHaveLength(200);
    expect(extract1688StructuredSkuImageRowsFromUnknown([{ skuProps: [{ values }] }], Number.POSITIVE_INFINITY)).toHaveLength(200);
  });

  it('keeps list/label/nested-image structured metadata support in the serialized page collector', () => {
    expect(
      extract1688StructuredSkuImageRowsFromUnknown([
        {
          saleProps: [
            {
              name: '规格',
              list: [
                {
                  label: '方8*40*100含挡板',
                  image: { url: '//cbu01.alicdn.com/img/ibank/list-label-nested.jpg' },
                },
              ],
            },
          ],
        },
      ]),
    ).toEqual([
      {
        specification: '方8*40*100含挡板',
        image: 'https://cbu01.alicdn.com/img/ibank/list-label-nested.jpg',
      },
    ]);
    const serializedCollector = collect1688Page.toString();
    expect(serializedCollector).toContain('object.list');
    expect(serializedCollector).toContain('partObject.label');
  });

  it('bounds an exhausted or cyclic structured root without starving a later root', () => {
    const exhausted: Record<string, unknown> = {};
    exhausted.self = exhausted;
    exhausted.skuProps = [
        {
          name: '规格',
          values: Array.from({ length: 10_050 }, (_, index) => ({ name: `无图规格-${index}` })),
        },
      ];

    expect(
      extract1688StructuredSkuImageRowsFromUnknown([
        exhausted,
        {
          saleProps: [
            {
              name: '规格',
              values: [
                {
                  name: '后续根规格',
                  img: 'https://cbu01.alicdn.com/img/ibank/later-root.jpg',
                },
              ],
            },
          ],
        },
      ]),
    ).toEqual([
      {
        specification: '后续根规格',
        image: 'https://cbu01.alicdn.com/img/ibank/later-root.jpg',
      },
    ]);
  });

  it('merges a table thumbnail only by one exact SKU property match', () => {
    const skus: ProductSku[] = [
      { properties: { 规格: '方8*40*100含挡板' } },
      { properties: { 规格: '方8*40*120含挡板' }, image: 'https://cbu01.alicdn.com/existing.jpg' },
      { properties: { 规格: '重复规格', 颜色: '白色' } },
      { properties: { 规格: '重复规格', 颜色: '黑色' } },
      { properties: { 规格: '冲突图片' } },
    ];
    const merged = merge1688SkuImagesIntoSkus(skus, [
      { specification: '方8*40*100含挡板', image: '//cbu01.alicdn.com/img/ibank/sku-100.jpg' },
      { specification: '方8*40*100', image: 'https://cbu01.alicdn.com/img/ibank/sub-string.jpg' },
      { specification: '方8*40*120含挡板', image: 'https://cbu01.alicdn.com/img/ibank/new.jpg' },
      { specification: '重复规格', image: 'https://cbu01.alicdn.com/img/ibank/ambiguous.jpg' },
      { specification: '冲突图片', image: 'https://cbu01.alicdn.com/img/ibank/conflict-a.jpg' },
      { specification: '冲突图片', image: 'https://cbu01.alicdn.com/img/ibank/conflict-b.jpg' },
    ]);

    expect(merged.map((sku) => sku.image)).toEqual([
      'https://cbu01.alicdn.com/img/ibank/sku-100.jpg',
      'https://cbu01.alicdn.com/existing.jpg',
      undefined,
      undefined,
      undefined,
    ]);
    expect(skus[0]?.image).toBeUndefined();
  });

  it('prefers structured full-list images and does not use DOM fallback after a structured conflict', () => {
    const merged = merge1688SkuImagesIntoSkus(
      [{ properties: { 规格: '结构化优先' } }, { properties: { 规格: '结构化冲突' } }],
      [
        { specification: '结构化优先', image: 'https://cbu01.alicdn.com/img/ibank/structured.jpg' },
        { specification: '结构化冲突', image: 'https://cbu01.alicdn.com/img/ibank/conflict-a.jpg' },
        { specification: '结构化冲突', image: 'https://cbu01.alicdn.com/img/ibank/conflict-b.jpg' },
      ],
      [
        { specification: '结构化优先', image: 'https://cbu01.alicdn.com/img/ibank/dom.jpg' },
        { specification: '结构化冲突', image: 'https://cbu01.alicdn.com/img/ibank/dom-only.jpg' },
      ],
    );

    expect(merged.map((sku) => sku.image)).toEqual([
      'https://cbu01.alicdn.com/img/ibank/structured.jpg',
      undefined,
    ]);
  });

  it('extracts the screenshot purchase-table descriptive columns and excludes commerce controls', () => {
    expect(
      extract1688SkuPropertyRowsFromUnknown([
        {
          headers: [' 产品规格 ', '螺纹公称 （M） （mm）', '公称长度\n(mm)', '适用范围', '价格 | 库存 (件)', '进货数量'],
          rows: [
            { cells: ['方8*40*100含挡板', '8', '100', '', '￥1.7 / 3990', '0'] },
            { cells: ['方8*40*120含挡板', '8', '120', '—', '￥1.8 / 4815', '0'] },
          ],
        },
      ]),
    ).toEqual([
      {
        specification: '方8*40*100含挡板',
        properties: {
          产品规格: '方8*40*100含挡板',
          '螺纹公称(M)(mm)': '8',
          '公称长度(mm)': '100',
        },
      },
      {
        specification: '方8*40*120含挡板',
        properties: {
          产品规格: '方8*40*120含挡板',
          '螺纹公称(M)(mm)': '8',
          '公称长度(mm)': '120',
        },
      },
    ]);
  });

  it('rejects packaging, ordinary, malformed, duplicate-header, and over-wide tables', () => {
    const cyclicRows: unknown[] = [];
    cyclicRows.push(cyclicRows);
    expect(
      extract1688SkuPropertyRowsFromUnknown([
        {
          contextText: '包装信息 商品件重尺',
          headers: ['产品规格', '长(cm)', '宽(cm)', '价格 | 库存'],
          rows: [{ cells: ['包装规格', '1', '1', '￥1 / 20'] }],
        },
        {
          headers: ['产品规格', '产品尺寸', '品名'],
          rows: [{ cells: ['普通规格', '10×20×30', '普通品名'] }],
        },
        {
          headers: ['产品规格', '产品规格', '价格 | 库存'],
          rows: [{ cells: ['重复头', '重复头', '￥1 / 20'] }],
        },
        {
          headers: ['产品规格', '', '价格 | 库存'],
          rows: [{ cells: ['空头', '8', '￥1 / 20'] }],
        },
        {
          headers: ['产品规格', '__proto__', '价格 | 库存'],
          rows: [{ cells: ['原型键', '污染值', '￥1 / 20'] }],
        },
        {
          headers: ['产品规格', 'constructor', '价格 | 库存'],
          rows: [{ cells: ['构造键', '污染值', '￥1 / 20'] }],
        },
        {
          headers: ['产品规格', ...Array.from({ length: 11 }, (_, index) => `属性${index}`), '价格 | 库存'],
          rows: [{ cells: ['过宽', ...Array.from({ length: 11 }, () => '值'), '￥1 / 20'] }],
        },
        {
          headers: ['产品规格', '公称长度(mm)', '价格 | 库存'],
          rows: [
            { cells: ['超长值', 'x'.repeat(201), '￥1 / 20'] },
            ...(cyclicRows as Array<{ cells?: unknown[] }>),
          ],
        },
      ]),
    ).toEqual([]);
  });

  it('caps inspected purchase-table rows at 200 and defines later-candidate reachability', () => {
    const rows = extract1688SkuPropertyRowsFromUnknown([
      {
        headers: ['产品规格', '公称长度(mm)', '价格 | 库存'],
        rows: Array.from({ length: 205 }, (_, index) => ({
          cells: [`规格${index}`, `${100 + index}`, `￥1 / ${1000 + index}`],
        })),
      },
    ]);
    expect(rows).toHaveLength(200);
    expect(rows[199]?.specification).toBe('规格199');

    const invalidRows = Array.from({ length: 10_050 }, () => ({ cells: ['', '100', '￥1 / 1000'] }));
    const validCandidate = {
      headers: ['产品规格', '公称长度(mm)', '购买数量'],
      rows: [{ cells: ['后续有效规格', '100', '0'] }],
    };
    expect(
      extract1688SkuPropertyRowsFromUnknown([
        { headers: ['产品规格', '公称长度(mm)', '价格 | 库存'], rows: invalidRows },
        validCandidate,
      ]),
    ).toEqual([]);
    expect(
      extract1688SkuPropertyRowsFromUnknown([
        { headers: ['产品规格', '公称长度(mm)', '价格 | 库存'], rows: invalidRows.slice(0, 199) },
        validCandidate,
      ]),
    ).toEqual([
      {
        specification: '后续有效规格',
        properties: { 产品规格: '后续有效规格', '公称长度(mm)': '100' },
      },
    ]);
    expect(
      extract1688SkuPropertyRowsFromUnknown([
        {
          contextText: '包装信息 商品件重尺',
          headers: ['产品规格', '公称长度(mm)', '价格 | 库存'],
          rows: invalidRows,
        },
        validCandidate,
      ]),
    ).toEqual([
      {
        specification: '后续有效规格',
        properties: { 产品规格: '后续有效规格', '公称长度(mm)': '100' },
      },
    ]);
  });

  it('replaces an equal generic 规格 atomically and merges only one exact non-conflicting row', () => {
    const skus: ProductSku[] = [
      { properties: { 规格: '方8*40*100含挡板' } },
      { properties: { 规格: '方8*40*120含挡板', 材质: '不锈钢' } },
      { properties: { 规格: '重复规格', 颜色: '白色' } },
      { properties: { 规格: '重复规格', 颜色: '黑色' } },
      { properties: { 规格: '冲突行' } },
      { properties: { 规格: '既有冲突', '公称长度(mm)': '999' } },
      { properties: { 型号: '规格值不同', 规格: '另一个值' } },
      { properties: { 规格: '已有产品规格冲突', 产品规格: '旧产品规格' } },
      { properties: { 规格: '真实规格', 产品尺寸: '尺寸偶合值' } },
    ];
    const merged = merge1688SkuPropertiesIntoSkus(skus, [
      {
        specification: '方8*40*100含挡板',
        properties: { 产品规格: '方8*40*100含挡板', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '方8*40*120含挡板',
        properties: { 产品规格: '方8*40*120含挡板', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '120' },
      },
      {
        specification: '方8*40*100',
        properties: { 产品规格: '方8*40*100', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '重复规格',
        properties: { 产品规格: '重复规格', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '冲突行',
        properties: { 产品规格: '冲突行', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '冲突行',
        properties: { 产品规格: '冲突行', '螺纹公称(M)(mm)': '10', '公称长度(mm)': '120' },
      },
      {
        specification: '既有冲突',
        properties: { 产品规格: '既有冲突', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '规格值不同',
        properties: { 产品规格: '规格值不同', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '已有产品规格冲突',
        properties: { 产品规格: '已有产品规格冲突', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '尺寸偶合值',
        properties: { 产品规格: '尺寸偶合值', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
    ]);

    expect(merged[0]?.properties).toEqual({
      产品规格: '方8*40*100含挡板',
      '螺纹公称(M)(mm)': '8',
      '公称长度(mm)': '100',
    });
    expect(Object.keys(merged[0]?.properties ?? {})).toHaveLength(3);
    expect(merged[1]?.properties).toEqual({
      材质: '不锈钢',
      产品规格: '方8*40*120含挡板',
      '螺纹公称(M)(mm)': '8',
      '公称长度(mm)': '120',
    });
    expect(merged.slice(2).map((sku) => sku.properties)).toEqual(skus.slice(2).map((sku) => sku.properties));
    expect(skus[0]?.properties).toEqual({ 规格: '方8*40*100含挡板' });
  });

  it('requires exactly one self-consistent specification property before merging', () => {
    const skus: ProductSku[] = [
      { properties: { 规格: '缺规格字段' } },
      { properties: { 规格: '规格值矛盾' } },
      { properties: { 规格: '重复规范头' } },
    ];
    const merged = merge1688SkuPropertiesIntoSkus(skus, [
      {
        specification: '缺规格字段',
        properties: { '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '规格值矛盾',
        properties: { 产品规格: '另一个规格', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '重复规范头',
        properties: {
          产品规格: '重复规范头',
          '产品 规格': '重复规范头',
          '公称长度(mm)': '100',
        },
      },
    ]);
    expect(merged.map((sku) => sku.properties)).toEqual(skus.map((sku) => sku.properties));
  });

  it('canonicalizes equal existing property keys but rejects conflicts and normalized-key ambiguity atomically', () => {
    const skus: ProductSku[] = [
      {
        properties: {
          规格: '同值规范化',
          '公称长度 （mm）': '100',
          '螺纹公称（M）（mm）': '8',
          材质: '不锈钢',
        },
      },
      { properties: { 规格: '异值拒绝', '公称长度 （mm）': '999', 材质: '不锈钢' } },
      {
        properties: {
          规格: '歧义拒绝',
          '公称长度 （mm）': '100',
          '公称长度(mm)': '100',
          材质: '不锈钢',
        },
      },
    ];
    const rows = [
      {
        specification: '同值规范化',
        properties: { 产品规格: '同值规范化', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '异值拒绝',
        properties: { 产品规格: '异值拒绝', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
      {
        specification: '歧义拒绝',
        properties: { 产品规格: '歧义拒绝', '螺纹公称(M)(mm)': '8', '公称长度(mm)': '100' },
      },
    ];
    const merged = merge1688SkuPropertiesIntoSkus(skus, rows);

    expect(merged[0]?.properties).toEqual({
      材质: '不锈钢',
      产品规格: '同值规范化',
      '螺纹公称(M)(mm)': '8',
      '公称长度(mm)': '100',
    });
    expect(merged[1]?.properties).toEqual(skus[1]?.properties);
    expect(merged[2]?.properties).toEqual(skus[2]?.properties);
    expect(skus[0]?.properties).toHaveProperty('公称长度 （mm）', '100');
  });

  it('keeps purchase-table property rules in the serialized page collector', () => {
    const serializedCollector = collect1688Page.toString();
    expect(serializedCollector).toContain('domSkuPropertyRows');
    expect(serializedCollector).toContain('sourceSpecificationName');
    expect(serializedCollector).toContain('descriptiveIndexes.length >= 2');
    expect(serializedCollector).toContain('assignments.size !== 1');
    expect(serializedCollector).toContain('购买数量|起订数量');
    expect(serializedCollector).toContain('packagingContextFor(table)');
    expect(serializedCollector).toContain('inspectedSkuPurchaseRows >= 200');
    expect(serializedCollector).toContain('__proto__|prototype|constructor');
    expect(serializedCollector).toContain('specificationEntries.length !== 1');
    expect(serializedCollector).toContain('existingByNormalizedName');
  });

  it('keeps product attributes from explicit DOM attribute contexts only', () => {
    expect(
      extract1688ProductAttributesFromUnknown({
        domCandidates: [
          { contextText: '商品属性', sourceHint: 'offer-attr-item', name: '材质', value: '橡胶' },
          { contextText: '产品参数', sourceHint: 'product-param-table', name: '形状', value: '圆形' },
          { contextText: '商品属性', sourceHint: 'de-feature-item', name: '用途', value: '密封' },
          { contextText: '产品参数', sourceHint: 'param-table', name: '表面处理', value: '光面' },
          { contextText: '普通表格', sourceHint: 'param-table', name: '9#橡胶塞', value: '9#橡胶塞' },
          { contextText: '产品规格', sourceHint: 'obj-content-table', name: '5#橡胶塞', value: '5#橡胶塞' },
          { contextText: '包装信息 商品件重尺', sourceHint: 'obj-content-table', name: '重量(g)', value: '2000' },
          { contextText: '普通表格', sourceHint: 'obj-content-table', name: '6#橡胶塞', value: '6#橡胶塞' },
        ],
      }),
    ).toEqual({ 材质: '橡胶', 形状: '圆形', 用途: '密封', 表面处理: '光面' });
  });

  it('does not reinterpret SKU, specification, packaging, price, or stock arrays as product attributes', () => {
    expect(
      extract1688ProductAttributesFromUnknown({
        structuredRoots: [
          {
            skuModel: {
              skuProps: [
                { name: '5#橡胶塞', value: '5#橡胶塞' },
                { name: '6#橡胶塞', value: '6#橡胶塞' },
              ],
            },
            specificationTable: [
              { name: '7#橡胶塞', value: '37*28*30' },
            ],
            packaging: [{ name: '重量(g)', value: 2200 }],
            priceTiers: [{ name: '起订量', value: 5 }],
            inventory: [{ name: '库存', value: 999 }],
          },
        ],
      }),
    ).toEqual({});
  });

  it('extracts semantic JSON attributes when attribute and SKU models coexist', () => {
    expect(
      extract1688ProductAttributesFromUnknown({
        structuredRoots: [
          {
            pageModel: {
              productAttributes: [
                { name: '材质', value: '硅橡胶' },
                { fname: '形状', vname: '圆柱形' },
              ],
              offerAttr: { 用途: '工业密封', 是否进口: false },
              offerParams: [
                { name: '加工定制', value: '否' },
              ],
              productFeatureList: [
                { attributeName: '工艺', value: '模压' },
              ],
              skuMap: {
                attributes: [{ name: '8#橡胶塞', value: '8#橡胶塞' }],
              },
            },
          },
        ],
      }),
    ).toEqual({
      材质: '硅橡胶',
      形状: '圆柱形',
      用途: '工业密封',
      是否进口: false,
      加工定制: '否',
      工艺: '模压',
    });
  });

  it('keeps response control fields out of semantic attribute maps', () => {
    expect(
      extract1688ProductAttributesFromUnknown({
        structuredRoots: [
          {
            attributes: {
              材质: '橡胶',
              status: 'success',
              success: true,
              message: 'ok',
              count: 1,
              total: 1,
              page: 1,
              pageSize: 20,
              hasMore: false,
            },
          },
        ],
      }),
    ).toEqual({ 材质: '橡胶' });
  });

  it('accepts a deep explicit attributeName/value shape outside generic arrays', () => {
    expect(
      extract1688ProductAttributesFromUnknown({
        structuredRoots: [
          { page: { payload: { detail: { attributeName: '适用范围', attributeValue: '机械设备' } } } },
        ],
      }),
    ).toEqual({ 适用范围: '机械设备' });
  });

  it('bounds cyclic and oversized traversal per root without blocking a later root', () => {
    const cyclic: Record<string, unknown> = {};
    cyclic.self = cyclic;
    cyclic.noise = Array.from({ length: 20 }, (_, index) => ({ nested: { value: `noise-${index}` } }));
    const attributes = extract1688ProductAttributesFromUnknown({
      structuredRoots: [
        cyclic,
        { nested: { attributeName: '颜色', attributeValue: '白色' } },
      ],
      nodeBudgetPerRoot: 8,
    });
    expect(attributes).toEqual({ 颜色: '白色' });

    const capped = extract1688ProductAttributesFromUnknown({
      structuredRoots: [
        {
          productAttributeList: Array.from({ length: 250 }, (_, index) => ({
            attributeName: `属性${index}`,
            attributeValue: `值${index}`,
          })),
        },
      ],
    });
    expect(Object.keys(capped)).toHaveLength(200);
  });

  it('prefers nested SKU prices over an offer-level fallback', () => {
    expect(extract1688SkuPriceFromBucket({ price: { value: '12.50' } })).toBe(12.5);
    expect(extract1688SkuPriceFromBucket({ price: { number: 13.8 } })).toBe(13.8);
    expect(
      extract1688SkuPriceFromBucket({ promotionPrices: { salePriceMoney: { value: '9.90' } } }),
    ).toBe(9.9);
    expect(extract1688SkuPriceFromBucket({ stock: 99 })).toBeUndefined();
  });

  it('parses price and quantity safely', () => {
    expect(parse1688Price('¥12.50')).toBe(12.5);
    expect(parse1688Price(0)).toBeUndefined();
    expect(parse1688Quantity('100')).toBe(100);
    expect(parse1688Quantity(-1)).toBe(0);
  });

  it('extracts semantic threshold-only ladder tiers without inventing end bounds', () => {
    expect(
      extractPriceTiersFromUnknown({
        tradeModel: {
          priceRange: [
            { beginAmount: 100, price: 11.8 },
            { beginAmount: 2, price: 12.5 },
          ],
        },
      }),
    ).toEqual([
      { beginAmount: 2, price: 12.5 },
      { beginAmount: 100, price: 11.8 },
    ]);
    expect(extractPriceTiersFromUnknown({ price: 9.9 })).toEqual([]);
    expect(extractPriceTiersFromUnknown({ offers: [{ beginAmount: 2, price: 12.5 }] })).toEqual([]);
  });

  it('validates explicit ladder intervals and rejects malformed or overlapping candidates as a whole', () => {
    expect(
      extractPriceTiersFromUnknown({
        priceRangeList: [
          { beginAmount: 2, endAmount: 9, price: 12.5 },
          { beginAmount: 10, endAmount: '', price: 11.8 },
          { beginAmount: 100, endAmount: null, price: 10.5 },
        ],
      }),
    ).toEqual([
      { beginAmount: 2, endAmount: 9, price: 12.5 },
      { beginAmount: 10, price: 11.8 },
      { beginAmount: 100, price: 10.5 },
    ]);
    expect(
      extractPriceTiersFromUnknown({ priceRange: [{ beginAmount: 10, endAmount: 9, price: 12.5 }] }),
    ).toEqual([]);
    expect(extractPriceTiersFromUnknown({ priceRange: [{ beginAmount: 2, price: '-12.5' }] })).toEqual([]);
    expect(
      extractPriceTiersFromUnknown({
        priceRange: [
          { beginAmount: 2, endAmount: 10, price: 12.5 },
          { beginAmount: 10, price: 11.8 },
        ],
      }),
    ).toEqual([]);
    expect(extractPriceTiersFromUnknown({ priceRange: [{ beginAmount: 2.5, price: 12.5 }] })).toEqual([]);
  });

  it('deduplicates identical tiers but rejects conflicting duplicate starts', () => {
    expect(
      extractPriceTiersFromUnknown({
        priceTiers: [
          { beginAmount: 2, price: 12.5 },
          { beginAmount: 2, price: 12.5 },
          { beginAmount: 100, price: 11.8 },
        ],
      }),
    ).toEqual([
      { beginAmount: 2, price: 12.5 },
      { beginAmount: 100, price: 11.8 },
    ]);
    expect(
      extractPriceTiersFromUnknown({
        priceTiers: [
          { beginAmount: 2, price: 12.5 },
          { beginAmount: 2, price: 11.8 },
        ],
      }),
    ).toEqual([]);
  });

  it('rejects a partially malformed tier candidate but can use a later independent valid candidate', () => {
    expect(
      extractPriceTiersFromUnknown({
        priceRange: [
          { name: '币种', value: 'CNY', status: 'active' },
          { beginAmount: 2, price: 12.5 },
          { beginAmount: 100 },
        ],
        nested: {
          ladderPrice: [
            { beginAmount: 5, price: 10 },
            { beginAmount: 50, price: 9 },
          ],
        },
      }),
    ).toEqual([
      { beginAmount: 5, price: 10 },
      { beginAmount: 50, price: 9 },
    ]);
  });

  it('does not salvage a nested ladder from rows of a rejected outer candidate', () => {
    expect(
      extractPriceTiersFromUnknown({
        priceRange: [
          { beginAmount: 2, price: 12.5 },
          {
            beginAmount: 2,
            price: 11.8,
            ladderPrice: [{ beginAmount: 5, price: 9.9 }],
          },
        ],
      }),
    ).toEqual([]);
  });

  it('bounds semantic candidate rows and charges them to the current root budget', () => {
    const oversized = Array.from({ length: 201 }, (_, index) => ({ beginAmount: index + 1, price: 10 }));
    expect(extractPriceTiersFromUnknown({ priceRange: oversized })).toEqual([]);
    expect(
      extractPriceTiersFromUnknown(
        [
          {
            priceRange: [
              { beginAmount: 2, price: 12.5 },
              { beginAmount: 10, price: 11.8 },
            ],
          },
          { priceTiers: [{ beginAmount: 3, price: 8.8 }] },
        ],
        3,
      ),
    ).toEqual([{ beginAmount: 3, price: 8.8 }]);
    expect(extractPriceTiersFromUnknown({ priceRange: [{ beginAmount: 3, price: 8.8 }] }, Infinity)).toEqual([
      { beginAmount: 3, price: 8.8 },
    ]);
  });

  it('resets cycle protection and a genuinely exhausted node budget for each independent root', () => {
    const exhaustedRoot: Record<string, unknown> = {};
    exhaustedRoot.self = exhaustedRoot;
    let cursor = exhaustedRoot;
    for (let index = 0; index < 12; index += 1) {
      const next: Record<string, unknown> = { metadata: { index } };
      cursor.next = next;
      cursor = next;
    }
    cursor.priceRange = [{ beginAmount: 1, price: 99 }];
    expect(
      extractPriceTiersFromUnknown(
        [exhaustedRoot, { tradeModel: { priceRange: [{ beginAmount: 3, price: 8.8 }] } }],
        4,
      ),
    ).toEqual([{ beginAmount: 3, price: 8.8 }]);
  });

  it('extracts min order quantity', () => {
    expect(extractMinOrderFromUnknown({ orderModel: { minOrderQuantity: 5 } })).toBe(5);
    expect(extractMinOrderFromUnknown({ moq: '3' })).toBe(3);
    expect(extractMinOrderFromUnknown({ tradeModel: { startQuantity: 8 } })).toBe(8);
  });

  it('does not recover product price or MOQ from an invalid tier container', () => {
    const invalidTier = {
      priceTiers: [
        { beginAmount: 2, price: 12.5 },
        { beginAmount: 2, price: 11.8 },
      ],
    };
    expect(extractPriceTiersFromUnknown(invalidTier)).toEqual([]);
    expect(extract1688OfferPriceFromUnknown(invalidTier)).toBeUndefined();
    expect(extractMinOrderFromUnknown(invalidTier)).toBeUndefined();
    expect(extract1688OfferPriceFromUnknown({ ...invalidTier, offerModel: { offerPrice: 9.9 } })).toBe(9.9);
    expect(extractMinOrderFromUnknown({ ...invalidTier, orderModel: { minOrderQuantity: 6 } })).toBe(6);
  });

  it('does not treat weights, dimensions, or title numbers as prices', () => {
    const noPrice = {
      title: '304 不锈钢测试商品',
      productPackInfo: {
        fields: {
          unitWeight: 39,
          netWeight: 304,
          length: 120,
        },
      },
    };
    expect(extract1688OfferPriceFromUnknown(noPrice)).toBeUndefined();
    expect(
      extract1688OfferPriceFromUnknown({
        ...noPrice,
        tradeModel: { fields: { price: 720 } },
      }),
    ).toBe(720);
  });

  it('parses common greater-than-delimited 1688 SKU keys into separate dimensions', () => {
    const dimensions = [
      { name: '颜色', values: ['蓝色【F106】'] },
      { name: '尺码', values: ['内长12【鞋底标12.5】'] },
    ];
    expect(parse1688SkuComboKey('蓝色【F106】>内长12【鞋底标12.5】', dimensions)).toEqual({
      颜色: '蓝色【F106】',
      尺码: '内长12【鞋底标12.5】',
    });
    expect(parse1688SkuComboKey('蓝色【F106】&gt;内长12【鞋底标12.5】', dimensions)).toEqual({
      颜色: '蓝色【F106】',
      尺码: '内长12【鞋底标12.5】',
    });
  });

  it('rejects DOM SKU values containing price or stock table noise', () => {
    expect(isValid1688SkuDimensionValue('黑色', '颜色')).toBe(true);
    expect(isValid1688SkuDimensionValue('M', '尺码')).toBe(true);
    expect(isValid1688SkuDimensionValue('颜色', '颜色')).toBe(false);
    expect(isValid1688SkuDimensionValue('库存299件', '颜色')).toBe(false);
    expect(isValid1688SkuDimensionValue('尺寸1.2mm ¥790 库存299件', '颜色')).toBe(false);
  });
});
