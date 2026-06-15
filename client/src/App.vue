<!-- App.vue - root layout. Header carries the ConnectionIndicator and a fleet
     status summary; the body is the FleetGrid. On mount it opens the fleet
     socket via the composable; the composable tears itself down on unmount. -->
<script setup lang="ts">
import { computed, onMounted } from 'vue'
import ConnectionIndicator from '@/components/ConnectionIndicator.vue'
import FleetGrid from '@/components/FleetGrid.vue'
import { WS_URL } from '@/lib/config'
import { useFleetSocket } from '@/composables/useFleetSocket'
import { useFleetStore } from '@/stores/fleet'

const store = useFleetStore()
const socket = useFleetSocket(WS_URL)

const summary = computed(() => store.statusCounts)

onMounted(() => {
  socket.connect()
})
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <h1>Live Fleet Grid</h1>
        <p class="tagline">Real-time tracked-entity grid over a WebSocket feed</p>
      </div>
      <ConnectionIndicator />
    </header>

    <nav class="summary">
      <span class="chip underway">Underway {{ summary.UNDERWAY }}</span>
      <span class="chip anchored">Anchored {{ summary.ANCHORED }}</span>
      <span class="chip moored">Moored {{ summary.MOORED }}</span>
    </nav>

    <main class="content">
      <FleetGrid />
    </main>
  </div>
</template>

<style scoped>
.app {
  max-width: 1100px;
  margin: 0 auto;
  padding: 1.5rem 1.25rem 3rem;
}
.topbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  border-bottom: 1px solid #1f2935;
  padding-bottom: 1rem;
}
.brand h1 {
  margin: 0;
  font-size: 1.35rem;
  letter-spacing: -0.01em;
  color: #f0f6fc;
}
.tagline {
  margin: 0.25rem 0 0;
  font-size: 0.82rem;
  color: #7d8896;
}
.summary {
  display: flex;
  gap: 0.5rem;
  margin: 1rem 0;
  flex-wrap: wrap;
}
.chip {
  font-size: 0.74rem;
  padding: 0.25rem 0.6rem;
  border-radius: 999px;
  border: 1px solid #1f2935;
  color: #adbac7;
}
.chip.underway { border-color: #214b2c; color: #3fb950; }
.chip.anchored { border-color: #4b3a16; color: #d29922; }
.chip.moored { border-color: #2a313a; color: #8b949e; }
.content { margin-top: 0.5rem; }
</style>
