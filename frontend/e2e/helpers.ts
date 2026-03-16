import type { Page, Route } from "@playwright/test";

interface MockUser {
  id: number;
  username: string;
  realName: string;
  role: string;
  roles: string[];
  permissions: string[];
  organizations: Array<{
    organizationType: string;
    organizationId: number;
    isPrimary: boolean;
  }>;
}

const defaultUser: MockUser = {
  id: 1,
  username: "root",
  realName: "Root Admin",
  role: "root",
  roles: ["root"],
  permissions: ["*"],
  organizations: [{ organizationType: "company", organizationId: 1, isPrimary: true }],
};

export async function setAuthedSession(page: Page, user: MockUser = defaultUser): Promise<void> {
  await page.addInitScript((sessionUser) => {
    window.sessionStorage.setItem("assessv2_token", "test-token");
    window.sessionStorage.setItem("assessv2_user", JSON.stringify(sessionUser));
    window.sessionStorage.setItem("assessv2_must_change_password", "false");
  }, user);
}

export async function mockProfile(page: Page, user: MockUser = defaultUser): Promise<void> {
  await page.route("**/api/system/profile", async (route) => {
    await ok(route, {
      user,
      mustChangePassword: false,
    });
  });
}

export async function ok(route: Route, data: unknown): Promise<void> {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      code: 200,
      message: "success",
      data,
    }),
  });
}
