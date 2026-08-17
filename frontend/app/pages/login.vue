<script setup lang="ts">
const auth = useAuthStore()

const email = ref('')
const status = ref<'idle' | 'sending' | 'sent' | 'error'>('idle')

async function onSubmit() {
  status.value = 'sending'
  try {
    await auth.requestMagicLink(email.value.trim())
    status.value = 'sent'
  } catch {
    status.value = 'error'
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-ground px-4 text-ink">
    <div class="w-full max-w-sm space-y-8">
      <div class="space-y-2 text-center">
        <h1 class="font-display text-2xl font-bold">
          Face Value
        </h1>
        <p class="text-sm text-ink-soft">
          Sign in with your email
        </p>
      </div>

      <form
        v-if="status !== 'sent'"
        class="space-y-3"
        @submit.prevent="onSubmit"
      >
        <div>
          <label
            for="email"
            class="block text-sm text-ink-soft"
          >Email</label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            placeholder="you@example.com"
            autocomplete="email"
            class="mt-1 w-full rounded-md bg-ground-deep px-3 py-2 text-sm text-ink ring-1 ring-line-strong"
          >
        </div>

        <button
          type="submit"
          :disabled="status === 'sending'"
          class="w-full rounded-full bg-terracotta-ink px-4 py-2 font-display text-sm font-semibold text-white transition-transform hover:-translate-y-0.5 disabled:opacity-50 disabled:hover:translate-y-0"
        >
          {{ status === 'sending' ? 'Sending…' : 'Send sign-in link' }}
        </button>

        <p
          v-if="status === 'error'"
          class="text-sm text-fail"
        >
          Something went wrong. Please try again.
        </p>
      </form>

      <p
        v-else
        class="text-center text-sm text-ink-soft"
      >
        If that email is allowed to sign in, a link has been sent. Check your
        inbox and click the link to continue.
      </p>
    </div>
  </div>
</template>
