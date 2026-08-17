<script setup lang="ts">
const auth = useAuthStore()
const route = useRoute()

const status = ref<'verifying' | 'error'>('verifying')

onMounted(async () => {
  const token = route.query.token

  if (typeof token !== 'string' || token === '') {
    status.value = 'error'
    return
  }

  try {
    await auth.verifyMagicLink(token)
    await navigateTo('/')
  } catch {
    status.value = 'error'
  }
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-ground px-4 text-ink">
    <div class="w-full max-w-sm space-y-8 text-center">
      <h1 class="font-display text-2xl font-bold">
        Face Value
      </h1>

      <p
        v-if="status === 'verifying'"
        class="text-sm text-ink-soft"
      >
        Signing you in…
      </p>

      <div
        v-else
        class="space-y-4"
      >
        <p class="text-sm text-fail">
          This sign-in link is invalid or has expired.
        </p>
        <NuxtLink
          to="/login"
          class="inline-block rounded-full bg-terracotta-ink px-4 py-2 font-display text-sm font-semibold text-white transition-transform hover:-translate-y-0.5"
        >
          Back to sign in
        </NuxtLink>
      </div>
    </div>
  </div>
</template>
