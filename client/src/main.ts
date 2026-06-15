// main.ts - application bootstrap. Creates the Vue app, installs Pinia, applies
// the global baseline styles, and mounts into #app.
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from '@/App.vue'
import '@/styles.css'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
