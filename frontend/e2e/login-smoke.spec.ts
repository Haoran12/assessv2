import { expect, test } from "@playwright/test";

test("login page smoke", async ({ page }) => {
  await page.goto("/login");

  await expect(page.locator(".login-card")).toBeVisible();
  await expect(page.locator("input").first()).toHaveValue("root");
  await expect(page.locator("input[type='password']")).toHaveValue("#2026@hdwl");
});
