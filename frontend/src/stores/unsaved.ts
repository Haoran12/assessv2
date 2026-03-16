import { computed, ref } from "vue";
import { defineStore } from "pinia";

type DirtyFlags = Record<string, true>;

export const useUnsavedStore = defineStore("unsaved", () => {
  const dirtyFlags = ref<DirtyFlags>({});

  const dirtySources = computed(() => Object.keys(dirtyFlags.value));
  const hasUnsavedChanges = computed(() => dirtySources.value.length > 0);

  function markDirty(source: string): void {
    const key = source.trim();
    if (!key) {
      return;
    }
    dirtyFlags.value = {
      ...dirtyFlags.value,
      [key]: true,
    };
  }

  function clearDirty(source: string): void {
    const key = source.trim();
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

  return {
    dirtySources,
    hasUnsavedChanges,
    markDirty,
    clearDirty,
    clearAll,
  };
});
