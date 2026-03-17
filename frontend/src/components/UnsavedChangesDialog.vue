<template>
  <el-dialog
    :model-value="state.visible"
    :title="state.title"
    width="460px"
    :show-close="false"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    @closed="handleClosed"
  >
    <p class="unsaved-message">{{ state.message }}</p>
    <template #footer>
      <div class="dialog-actions">
        <el-button @click="handleReturn">{{ state.returnButtonText }}</el-button>
        <el-button type="danger" plain @click="handleDiscard">
          {{ state.discardButtonText }}
        </el-button>
        <el-button type="primary" @click="handleSave">
          {{ state.saveButtonText }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { cancelUnsavedDialog, resolveUnsavedDialog, useUnsavedDialogState } from "@/services/unsaved-dialog";

const state = useUnsavedDialogState();

function handleReturn(): void {
  resolveUnsavedDialog("return_editing");
}

function handleDiscard(): void {
  resolveUnsavedDialog("discard_changes");
}

function handleSave(): void {
  resolveUnsavedDialog("save_and_leave");
}

function handleClosed(): void {
  if (!state.visible) {
    cancelUnsavedDialog();
  }
}
</script>

<style scoped>
.unsaved-message {
  margin: 0;
  line-height: 1.6;
  color: #303133;
}

.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
