import { expect, test } from "@playwright/test";
import { mockProfile, ok, setAuthedSession } from "./helpers";

test("assessment page supports create year and status transition interactions", async ({ page }) => {
  await setAuthedSession(page);
  await mockProfile(page);

  const years = [
    {
      id: 1,
      year: 2025,
      yearName: "2025骞村害鑰冩牳",
      status: "preparing",
      description: "",
    },
  ];

  const periodsByYear: Record<number, unknown[]> = {
    1: [
      { id: 11, yearId: 1, periodCode: "Q1", periodName: "绗竴瀛ｅ害", status: "preparing" },
      { id: 12, yearId: 1, periodCode: "Q2", periodName: "绗簩瀛ｅ害", status: "preparing" },
      { id: 13, yearId: 1, periodCode: "Q3", periodName: "绗笁瀛ｅ害", status: "preparing" },
      { id: 14, yearId: 1, periodCode: "Q4", periodName: "绗洓瀛ｅ害", status: "preparing" },
      { id: 15, yearId: 1, periodCode: "YEAR_END", periodName: "骞寸粓鑰冩牳", status: "preparing" },
    ],
  };

  const objectsByYear: Record<number, unknown[]> = {
    1: [{ id: 100, yearId: 1, objectType: "team", objectCategory: "company", targetId: 1, targetType: "organization", objectName: "娴嬭瘯鍏徃", isActive: true }],
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
        year: 2027,
        yearName: "2027骞村害鑰冩牳",
        status: "preparing",
        description: "",
      };
      years.unshift(created);
      periodsByYear[2] = [
        { id: 21, yearId: 2, periodCode: "Q1", periodName: "绗竴瀛ｅ害", status: "preparing" },
        { id: 22, yearId: 2, periodCode: "Q2", periodName: "绗簩瀛ｅ害", status: "preparing" },
        { id: 23, yearId: 2, periodCode: "Q3", periodName: "绗笁瀛ｅ害", status: "preparing" },
        { id: 24, yearId: 2, periodCode: "Q4", periodName: "绗洓瀛ｅ害", status: "preparing" },
        { id: 25, yearId: 2, periodCode: "YEAR_END", periodName: "骞寸粓鑰冩牳", status: "preparing" },
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

  await expect(page.getByText("骞村害绠＄悊")).toBeVisible();
  await expect(page.getByText("绗竴瀛ｅ害")).toBeVisible();

  await page.getByRole("button", { name: "鍒涘缓骞村害" }).click();
  await expect(page.getByText("鍒涘缓鑰冩牳骞村害")).toBeVisible();
  await page.getByRole("button", { name: "鍒涘缓" }).click();

  await expect(page.getByText("骞村害鍒涘缓鎴愬姛锛岃嚜鍔ㄧ敓鎴?5 涓懆鏈?)).toBeVisible();
  await expect(page.getByText("2027骞村害鑰冩牳")).toBeVisible();
});

