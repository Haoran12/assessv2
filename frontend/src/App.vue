<template>
  <RouterView />
  <UnsavedChangesDialog />
</template>

<script setup lang="ts">
import UnsavedChangesDialog from "@/components/UnsavedChangesDialog.vue";
</script>
