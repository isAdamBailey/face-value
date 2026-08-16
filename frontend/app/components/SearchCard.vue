<script setup lang="ts">
import type { Search } from '~/stores/searches'

const props = defineProps<{ search: Search }>()
const emit = defineEmits<{ retry: [id: string] }>()

const { format: formatMoney } = useMoney()
const { format: formatRelativeTime } = useRelativeTime()

const isTerminal = computed(() => props.search.status === 'complete' || props.search.status === 'failed')
const isFailed = computed(() => props.search.status === 'failed')
const isComplete = computed(() => props.search.status === 'complete')
const titleText = computed(() => {
  if (props.search.title) {
    return props.search.title
  }
  return isFailed.value ? 'Unidentified item' : 'Identifying…'
})
</script>

<template>
  <NuxtLink
    :to="isTerminal ? `/search/${search.id}` : undefined"
    class="group relative flex flex-col overflow-hidden rounded-lg bg-neutral-900 ring-1 ring-neutral-800 transition-colors"
    :class="[isTerminal ? 'hover:ring-neutral-700' : 'cursor-default', { 'opacity-60': isFailed }]"
  >
    <div class="aspect-square w-full overflow-hidden bg-neutral-800">
      <div
        v-if="!search.image_url"
        class="h-full w-full animate-pulse bg-neutral-700"
      />
      <img
        v-else
        :src="search.image_url"
        :alt="search.title || 'Uploaded item'"
        class="h-full w-full object-cover"
        :class="{ grayscale: isFailed }"
      >
    </div>

    <div class="flex flex-1 flex-col gap-1 p-3">
      <p class="truncate text-sm font-medium text-neutral-100">
        {{ titleText }}
      </p>

      <p v-if="isComplete" class="text-lg font-semibold text-emerald-400">
        {{ formatMoney(search.price_trimmed_mean, search.currency) }}
      </p>
      <p v-else-if="isFailed" class="text-sm text-red-400">
        Couldn't complete this search
      </p>
      <p v-else-if="search.still_working" class="text-sm text-neutral-400">
        Still working — check back soon
      </p>
      <p v-else class="text-sm text-neutral-400">
        Working…
      </p>

      <div class="mt-auto flex items-center justify-between text-xs text-neutral-500">
        <span v-if="isComplete && search.comp_count !== undefined">
          {{ search.comp_count }} listing{{ search.comp_count === 1 ? '' : 's' }}
        </span>
        <span v-else />
        <span>{{ formatRelativeTime(search.created_at) }}</span>
      </div>

      <button
        v-if="isFailed || search.still_working"
        type="button"
        class="self-start text-xs font-medium text-emerald-400 hover:text-emerald-300"
        @click.prevent="emit('retry', search.id)"
      >
        Retry
      </button>
    </div>
  </NuxtLink>
</template>
