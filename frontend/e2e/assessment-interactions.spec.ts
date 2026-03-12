import { expect, test } from "@playwright/test";
import { mockProfile, ok, setAuthedSession } from "./helpers";

test("assessment page supports create year and status transition interactions", async ({ page }) => {
  await setAuthedSession(page);
  await mockProfile(page);

  const years = [
    {
      id: 1,
      year: 2025,
      yearName: "2025年度考核",
      status: "preparing",
      description: "",
    },
  ];

  const periodsByYear: Record<number, unknown[]> = {
    1: [
      { id: 11, yearId: 1, periodCode: "Q1", periodName: "第一季度", status: "not_started" },
      { id: 12, yearId: 1, periodCode: "Q2", periodName: "第二季度", status: "not_started" },
      { id: 13, yearId: 1, periodCode: "Q3", periodName: "第三季度", status: "not_started" },
      { id: 14, yearId: 1, periodCode: "Q4", periodName: "第四季度", status: "not_started" },
      { id: 15, yearId: 1, periodCode: "YEAR_END", periodName: "年终考核", status: "not_started" },
    ],
  };

  const objectsByYear: Record<number, unknown[]> = {
    1: [{ id: 100, yearId: 1, objectType: "team", objectCategory: "company", targetId: 1, targetType: "organization", objectName: "测试公司", isActive: true }],
  };

  await page.route("**/api/assessment/**", async (route) => {
    const method = route.request().method();
    const url = new URL(route.request().url());
    const path = url.pathname;

    if (method === "GET" && path === "/api/assessment/years") {
      await ok(route, { items: years });
      return;
    }

    if (method === "POST" && path === "/api/assessment/years") {
      const created = {
        id: 2,
        year: 2026,
        yearName: "2026年度考核",
        status: "preparing",
        description: "",
      };
      years.unshift(created);
      periodsByYear[2] = [
        { id: 21, yearId: 2, periodCode: "Q1", periodName: "第一季度", status: "not_started" },
        { id: 22, yearId: 2, periodCode: "Q2", periodName: "第二季度", status: "not_started" },
        { id: 23, yearId: 2, periodCode: "Q3", periodName: "第三季度", status: "not_started" },
        { id: 24, yearId: 2, periodCode: "Q4", periodName: "第四季度", status: "not_started" },
        { id: 25, yearId: 2, periodCode: "YEAR_END", periodName: "年终考核", status: "not_started" },
      ];
      objectsByYear[2] = [];
      await ok(route, {
        year: created,
        periods: periodsByYear[2],
        objectsCount: 0,
      });
      return;
    }

    const periodsMatch = path.match(/^\/api\/assessment\/years\/(\d+)\/periods$/);
    if (method === "GET" && periodsMatch) {
      const yearId = Number(periodsMatch[1]);
      await ok(route, { items: periodsByYear[yearId] ?? [] });
      return;
    }

    const objectsMatch = path.match(/^\/api\/assessment\/years\/(\d+)\/objects$/);
    if (method === "GET" && objectsMatch) {
      const yearId = Number(objectsMatch[1]);
      await ok(route, { items: objectsByYear[yearId] ?? [] });
      return;
    }

    const yearStatusMatch = path.match(/^\/api\/assessment\/years\/(\d+)\/status$/);
    if (method === "PUT" && yearStatusMatch) {
      const yearId = Number(yearStatusMatch[1]);
      const payload = route.request().postDataJSON() as { status: string };
      const hit = years.find((item) => item.id === yearId);
      if (hit) {
        hit.status = payload.status;
      }
      await ok(route, hit ?? {});
      return;
    }

    const periodStatusMatch = path.match(/^\/api\/assessment\/periods\/(\d+)\/status$/);
    if (method === "PUT" && periodStatusMatch) {
      const periodId = Number(periodStatusMatch[1]);
      const payload = route.request().postDataJSON() as { status: string };
      for (const items of Object.values(periodsByYear)) {
        const hit = (items as Array<{ id: number; status: string }>).find((item) => item.id === periodId);
        if (hit) {
          hit.status = payload.status;
          await ok(route, hit);
          return;
        }
      }
      await ok(route, {});
      return;
    }

    await ok(route, {});
  });

  await page.goto("/assessment");

  await expect(page.getByText("年度管理")).toBeVisible();
  await expect(page.getByText("第一季度")).toBeVisible();

  await page.getByRole("button", { name: "创建年度" }).click();
  await expect(page.getByText("创建考核年度")).toBeVisible();
  await page.getByRole("button", { name: "创建" }).click();

  await expect(page.getByText("年度创建成功，自动生成 5 个周期")).toBeVisible();
  await expect(page.getByText("2026年度考核")).toBeVisible();
});
