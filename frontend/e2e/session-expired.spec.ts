import { expect, test } from "@playwright/test";

test("show session expired hint on login page", async ({ page }) => {
  await page.addInitScript(() => {
    window.sessionStorage.setItem("assessv2_session_expired", "1");
  });

  await page.goto("/login");

  await expect(page.getByText("登录已过期，请重新登录")).toBeVisible();
  await expect(page.getByRole("button", { name: "登录" })).toBeVisible();
});
