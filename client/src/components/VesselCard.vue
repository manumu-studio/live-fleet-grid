<!-- VesselCard.vue - one tracked entity. Shows name, type, a status dot, and
     live lat/lng. A CSS flash on any data change makes "liveness" visible. -->
<script setup lang="ts">
import { ref, watch } from 'vue'
import type { VesselCardProps } from './VesselCard.types'
import type { VesselStatus } from '@/lib/messages'

const props = defineProps<VesselCardProps>()

// Toggle a transient highlight class whenever this vessel's data changes.
const flash = ref(false)
let flashTimer: ReturnType<typeof setTimeout> | null = null

watch(
  () => [props.vessel.lat, props.vessel.lng, props.vessel.status],
  () => {
    flash.value = true
    if (flashTimer !== null) clearTimeout(flashTimer)
    flashTimer = setTimeout(() => {
      flash.value = false
    }, 600)
  },
)

const STATUS_LABEL: Record<VesselStatus, string> = {
  UNDERWAY: 'Underway',
  MOORED: 'Moored',
  ANCHORED: 'Anchored',
}

function fmt(n: number): string {
  return n.toFixed(3)
}
</script>

<template>
  <article class="card" :class="[`status-${vessel.status.toLowerCase()}`, { flash }]">
    <header class="card-head">
      <span class="dot" :aria-label="STATUS_LABEL[vessel.status]" />
      <h3 class="name">{{ vessel.name }}</h3>
      <span class="type">{{ vessel.vesselType }}</span>
    </header>
    <p class="status">{{ STATUS_LABEL[vessel.status] }}</p>
    <dl class="coords">
      <div><dt>LAT</dt><dd>{{ fmt(vessel.lat) }}</dd></div>
      <div><dt>LNG</dt><dd>{{ fmt(vessel.lng) }}</dd></div>
    </dl>
  </article>
</template>

<style scoped>
.card {
  background: #11161d;
  border: 1px solid #1f2935;
  border-radius: 10px;
  padding: 0.85rem 0.95rem;
  transition: box-shadow 0.6s ease, border-color 0.6s ease, background 0.6s ease;
}
.card.flash {
  border-color: #2f81f7;
  background: #131c28;
  box-shadow: 0 0 0 1px #2f81f7, 0 0 18px rgba(47, 129, 247, 0.35);
  transition: none;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.name {
  font-size: 0.95rem;
  margin: 0;
  font-weight: 600;
  color: #e6edf3;
}
.type {
  margin-left: auto;
  font-size: 0.7rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #7d8896;
}
.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #7d8896;
  flex: none;
}
.status-underway .dot { background: #3fb950; box-shadow: 0 0 8px #3fb95066; }
.status-anchored .dot { background: #d29922; box-shadow: 0 0 8px #d2992266; }
.status-moored .dot   { background: #8b949e; }
.status {
  margin: 0.5rem 0 0.6rem;
  font-size: 0.78rem;
  color: #adbac7;
}
.coords {
  display: flex;
  gap: 1.25rem;
  margin: 0;
}
.coords div { display: flex; flex-direction: column; gap: 2px; }
.coords dt {
  font-size: 0.6rem;
  letter-spacing: 0.08em;
  color: #6e7681;
}
.coords dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
  font-size: 0.85rem;
  color: #d1d9e0;
}
</style>
