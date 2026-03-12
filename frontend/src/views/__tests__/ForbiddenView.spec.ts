import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ForbiddenView from "../ForbiddenView.vue";

const pushMock = vi.fn();

vi.mock("vue-router", () => ({
  useRouter: () => ({
    push: pushMock,
  }),
}));

describe("ForbiddenView", () => {
  beforeEach(() => {
    pushMock.mockReset();
  });

  it("navigates to /dashboard when click button", async () => {
    const wrapper = mount(ForbiddenView, {
      global: {
        stubs: {
          "el-result": {
            template: '<div class="el-result"><slot name="extra" /></div>',
          },
          "el-button": {
            template: '<button type="button" @click="$emit(\'click\')"><slot /></button>',
          },
        },
      },
    });

    await wrapper.get("button").trigger("click");
    expect(pushMock).toHaveBeenCalledWith("/dashboard");
  });
});
