import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const route = "location ~ ^/api/v1/products/[^/]+/ai/ozon-attribute-suggestions$";

function read(path: string) {
  return readFileSync(new URL(`../../${path}`, import.meta.url), "utf8");
}

describe("Ozon AI attribute suggestion proxy timeout", () => {
  it.each([
    ["admin/nginx.conf", 1],
    ["deploy/nginx/trademind-staging.conf", 1],
    ["deploy/nginx/trademind.conf", 2],
  ] as const)("keeps 900 seconds scoped to the exact route in %s", (path, expected) => {
    const source = read(path);
    expect(source.split(route)).toHaveLength(expected + 1);
    expect(source.match(/proxy_read_timeout 900s;/g)).toHaveLength(expected);
    for (const block of source.split(route).slice(1)) {
      expect(block.slice(0, block.indexOf("}")).toString()).toContain(
        "proxy_read_timeout 900s;",
      );
    }
  });

  it("leaves the production generic API timeout unchanged", () => {
    expect(read("deploy/nginx/trademind.conf")).toContain(
      "proxy_read_timeout 300s;",
    );
  });
});
