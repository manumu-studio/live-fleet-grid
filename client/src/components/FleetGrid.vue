<!-- FleetGrid.vue - responsive CSS grid of VesselCards. Pulls the sorted
     vessel list from the store and keys each card by vessel id so Vue patches
     in place (no remount, so the per-card flash animation works correctly). -->
<script setup lang="ts">
import { useFleetStore } from '@/stores/fleet'
import VesselCard from './VesselCard.vue'

const store = useFleetStore()
</script>

<template>
  <section v-if="store.vesselCount > 0" class="grid">
    <VesselCard v-for="vessel in store.vesselList" :key="vessel.id" :vessel="vessel" />
  </section>
  <p v-else class="empty">Waiting for the first snapshot…</p>
</template>

<style scoped>
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 0.85rem;
}
.empty {
  color: #6e7681;
  font-size: 0.9rem;
  padding: 2rem 0;
}
</style>
