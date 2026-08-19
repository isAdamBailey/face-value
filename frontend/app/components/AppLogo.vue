<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 30 })
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    fill="none"
    aria-hidden="true"
    class="shrink-0"
  >
    <rect
      x="5"
      y="5"
      width="23"
      height="23"
      rx="7"
      transform="rotate(-8 16.5 16.5)"
      fill="var(--color-terracotta-ink)"
    />
    <circle
      cx="12.4"
      cy="11.2"
      r="2.6"
      fill="var(--color-ground)"
    />
  </svg>
</template>
