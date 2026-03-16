<template>
  <div class="system-overview-view">
    <ResultOverviewView />
    <AssessmentView />
  </div>
</template>

<script setup lang="ts">
import AssessmentView from "@/views/AssessmentView.vue";
import ResultOverviewView from "@/views/ResultOverviewView.vue";
</script>

<style scoped>
.system-overview-view {
  display: grid;
  gap: 16px;
}
</style>
