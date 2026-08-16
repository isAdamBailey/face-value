<script setup lang="ts">
const auth = useAuthStore()
</script>

<template>
  <main class="flex min-h-screen items-center justify-center bg-neutral-950 text-neutral-100">
    <div class="text-center">
      <h1 class="text-2xl font-semibold">
        Face Value
      </h1>
      <p
        v-if="auth.user"
        class="mt-2 text-sm text-neutral-400"
      >
        Signed in as {{ auth.user.email }}
      </p>
    </div>
  </main>
</template>
