import { watch } from "vue";
import { resolveUnsavedBeforeLeave } from "@/guards/unsaved";
import { useUnsavedStore } from "@/stores/unsaved";
import { ExitSystem, SetCloseGuard } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

const isBrowser = typeof window !== "undefined";
const isDesktopRuntime = typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("wails");

let initialized = false;

export function setupUnsavedCloseGuard(): void {
  if (initialized) {
    return;
  }
  initialized = true;

  const unsavedStore = useUnsavedStore();

  if (isBrowser) {
    window.addEventListener("beforeunload", (event) => {
      if (!unsavedStore.hasUnsavedChanges) {
        return;
      }
      event.preventDefault();
      event.returnValue = "";
    });
  }

  if (!isDesktopRuntime) {
    return;
  }

  watch(
    () => unsavedStore.hasUnsavedChanges,
    (hasUnsavedChanges) => {
      void SetCloseGuard(hasUnsavedChanges).catch(() => undefined);
    },
    { immediate: true },
  );

  let handlingCloseRequest = false;
  EventsOn("app:close-requested", async () => {
    if (handlingCloseRequest) {
      return;
    }
    handlingCloseRequest = true;
    try {
      const allowed = await resolveUnsavedBeforeLeave({
        title: "未保存改动提醒",
        message: "检测到存在未保存改动，关闭系统前请选择下一步操作。",
      });
      if (!allowed) {
        return;
      }
      await ExitSystem();
    } finally {
      handlingCloseRequest = false;
    }
  });
}
