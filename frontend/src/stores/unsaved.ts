import { computed, ref } from "vue";
import { defineStore } from "pinia";

type DirtyFlags = Record<string, true>;
type SourceMetaMap = Record<string, DirtySourceMeta>;

export type DirtySaveHandler = () => Promise<boolean | void> | boolean | void;

export interface DirtySourceMeta {
  label?: string;
  save?: DirtySaveHandler;
}

export const useUnsavedStore = defineStore("unsaved", () => {
  const dirtyFlags = ref<DirtyFlags>({});
  const sourceMeta = ref<SourceMetaMap>({});

  const dirtySources = computed(() => Object.keys(dirtyFlags.value));
  const hasUnsavedChanges = computed(() => dirtySources.value.length > 0);
  const dirtySourceEntries = computed(() =>
    dirtySources.value.map((source) => ({
      source,
      label: sourceMeta.value[source]?.label || source,
      canAutoSave: typeof sourceMeta.value[source]?.save === "function",
    })),
  );

  function normalizeSource(source: string): string {
    return source.trim();
  }

  function updateSourceMeta(source: string, patch: DirtySourceMeta): void {
    const key = normalizeSource(source);
    if (!key) {
      return;
    }
    sourceMeta.value = {
      ...sourceMeta.value,
      [key]: {
        ...sourceMeta.value[key],
        ...patch,
      },
    };
  }

  function setSourceMeta(source: string, meta: DirtySourceMeta): void {
    updateSourceMeta(source, meta);
  }

  function clearSourceMeta(source: string): void {
    const key = normalizeSource(source);
    if (!key || !sourceMeta.value[key]) {
      return;
    }
    const nextMeta: SourceMetaMap = { ...sourceMeta.value };
    delete nextMeta[key];
    sourceMeta.value = nextMeta;
  }

  function markDirty(source: string, meta?: DirtySourceMeta): void {
    const key = normalizeSource(source);
    if (!key) {
      return;
    }
    if (meta) {
      updateSourceMeta(key, meta);
    }
    dirtyFlags.value = {
      ...dirtyFlags.value,
      [key]: true,
    };
  }

  function clearDirty(source: string): void {
    const key = normalizeSource(source);
    if (!key || !dirtyFlags.value[key]) {
      return;
    }
    const nextFlags: DirtyFlags = { ...dirtyFlags.value };
    delete nextFlags[key];
    dirtyFlags.value = nextFlags;
  }

  function clearAll(): void {
    dirtyFlags.value = {};
  }

  function unregisterSource(source: string): void {
    clearDirty(source);
    clearSourceMeta(source);
  }

  function canAutoSave(source: string): boolean {
    const key = normalizeSource(source);
    if (!key) {
      return false;
    }
    return typeof sourceMeta.value[key]?.save === "function";
  }

  async function saveDirtySources(targetSources?: string[]): Promise<{
    saved: string[];
    failed: string[];
  }> {
    const sources = (targetSources ?? dirtySources.value).filter((source) => dirtyFlags.value[source]);
    const saved: string[] = [];
    const failed: string[] = [];

    for (const source of sources) {
      const handler = sourceMeta.value[source]?.save;
      if (!handler) {
        failed.push(source);
        continue;
      }
      try {
        const result = await handler();
        if (result === false) {
          failed.push(source);
          continue;
        }
      } catch (_error) {
        failed.push(source);
        continue;
      }
      if (dirtyFlags.value[source]) {
        failed.push(source);
        continue;
      }
      saved.push(source);
    }

    return { saved, failed };
  }

  return {
    dirtySources,
    dirtySourceEntries,
    hasUnsavedChanges,
    markDirty,
    clearDirty,
    clearAll,
    setSourceMeta,
    unregisterSource,
    canAutoSave,
    saveDirtySources,
  };
});
