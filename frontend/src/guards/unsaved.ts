import { ElMessage } from "element-plus";
import { openUnsavedDialog } from "@/services/unsaved-dialog";
import { useUnsavedStore } from "@/stores/unsaved";

interface ResolveUnsavedOptions {
  title?: string;
  message?: string;
}

let pendingResolvePromise: Promise<boolean> | null = null;

function failedSourceText(failedSources: string[]): string {
  const store = useUnsavedStore();
  const labels = failedSources.map((source) => {
    const hit = store.dirtySourceEntries.find((item) => item.source === source);
    return hit?.label || source;
  });
  return labels.join("、");
}

export async function resolveUnsavedBeforeLeave(options?: ResolveUnsavedOptions): Promise<boolean> {
  const store = useUnsavedStore();
  if (!store.hasUnsavedChanges) {
    return true;
  }

  if (pendingResolvePromise) {
    return pendingResolvePromise;
  }

  pendingResolvePromise = (async () => {
    try {
      const decision = await openUnsavedDialog({
        title: options?.title,
        message: options?.message,
      });

      if (decision === "return_editing") {
        return false;
      }

      if (decision === "discard_changes") {
        store.clearAll();
        return true;
      }

      const { failed } = await store.saveDirtySources();
      if (failed.length > 0) {
        ElMessage.warning(`以下改动未保存成功：${failedSourceText(failed)}`);
        return false;
      }
      if (store.hasUnsavedChanges) {
        ElMessage.warning("仍存在未保存改动，请先处理后再离开。");
        return false;
      }
      return true;
    } finally {
      pendingResolvePromise = null;
    }
  })();

  return pendingResolvePromise;
}
