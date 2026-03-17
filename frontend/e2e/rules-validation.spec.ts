import { expect, test } from "@playwright/test";
import { mockProfile, ok, setAuthedSession } from "./helpers";

test("rules page checks weight sum before submit", async ({ page }) => {
  await setAuthedSession(page);
  await mockProfile(page);

  await page.route("**/api/rules**", async (route) => {
    const method = route.request().method();
    const url = new URL(route.request().url());
    if (method === "GET" && url.pathname === "/api/rules") {
      await ok(route, { items: [] });
      return;
    }
    await ok(route, {});
  });

  await page.route("**/api/rules/templates**", async (route) => {
    await ok(route, { items: [] });
  });

  await page.goto("/rules");

  await expect(page.getByText("规则配置（M3）")).toBeVisible();

  await page.getByRole("spinbutton").first().fill("2027");
  await page.getByRole("button", { name: "新建规则" }).click();

  await expect(page.getByText("新建规则")).toBeVisible();

  const ruleNameInput = page.locator(".el-dialog").getByRole("textbox").first();
  await ruleNameInput.fill("E2E 权重校验规则");
  await page.locator(".el-dialog").getByRole("button", { name: "保存" }).click();

  await expect(page.getByText("参与折算的模块权重和必须等于 1.0000")).toBeVisible();
});
