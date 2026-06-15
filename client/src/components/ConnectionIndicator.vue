<!-- ConnectionIndicator.vue - connection-health readout: a colored dot, the
     current connection status text, and the running dropped-frame count.
     Reads straight from the Pinia store; holds no transport logic. -->
<script setup lang="ts">
import { computed } from 'vue'
import { useFleetStore } from '@/stores/fleet'
import type { ConnectionStatus } from '@/stores/fleet'

const store = useFleetStore()

const LABEL: Record<ConnectionStatus, string> = {
  connecting: 'Connecting',
  live: 'Live',
  reconnecting: 'Reconnecting',
  closed: 'Closed',
}

const statusLabel = computed(() => LABEL[store.connectionStatus])
</script>

<template>
  <div class="indicator" :class="`is-${store.connectionStatus}`">
    <span class="pulse" />
    <span class="label">{{ statusLabel }}</span>
    <span class="sep">·</span>
    <span class="meta">{{ store.vesselCount }} vessels</span>
    <span class="sep">·</span>
    <span class="meta">seq {{ store.lastSeq ?? '-' }}</span>
    <span class="sep">·</span>
    <span class="meta dropped">{{ store.droppedFrames }} dropped</span>
  </div>
</template>

<style scoped>
.indicator {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.82rem;
  color: #adbac7;
}
.pulse {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #6e7681;
  flex: none;
}
.label { font-weight: 600; color: #e6edf3; }
.sep { color: #404a56; }
.meta { font-variant-numeric: tabular-nums; }
.dropped { color: #d29922; }

.is-live .pulse {
  background: #3fb950;
  box-shadow: 0 0 0 0 rgba(63, 185, 80, 0.7);
  animation: ping 1.6s ease-out infinite;
}
.is-connecting .pulse,
.is-reconnecting .pulse {
  background: #d29922;
  animation: blink 1s step-end infinite;
}
.is-closed .pulse { background: #f85149; }

@keyframes ping {
  0% { box-shadow: 0 0 0 0 rgba(63, 185, 80, 0.6); }
  70% { box-shadow: 0 0 0 8px rgba(63, 185, 80, 0); }
  100% { box-shadow: 0 0 0 0 rgba(63, 185, 80, 0); }
}
@keyframes blink {
  50% { opacity: 0.35; }
}
</style>
