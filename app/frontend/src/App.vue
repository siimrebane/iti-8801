<script setup>
import { onMounted, ref } from 'vue'

const notes = ref([])
const text = ref('')
const error = ref('')
const version = ref(null)

async function load() {
  error.value = ''
  try {
    const r = await fetch('/api/notes')
    if (!r.ok) throw new Error('GET /api/notes: HTTP ' + r.status)
    notes.value = await r.json()
  } catch (e) {
    error.value = e.message
  }
}

async function add() {
  const t = text.value.trim()
  if (!t) return
  error.value = ''
  try {
    const r = await fetch('/api/notes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: t }),
    })
    if (r.status !== 201) throw new Error('POST /api/notes: HTTP ' + r.status)
    text.value = ''
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function remove(id) {
  error.value = ''
  try {
    const r = await fetch('/api/notes/' + id, { method: 'DELETE' })
    if (r.status !== 204) throw new Error('DELETE /api/notes/' + id + ': HTTP ' + r.status)
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  await load()
  try {
    const r = await fetch('/api/version')
    if (r.ok) version.value = await r.json()
  } catch (e) {
    /* the footer stays empty */
  }
})
</script>

<template>
  <main>
    <h1>Notes</h1>
    <p class="sub">A note travels from this page through the frontend tier to the backend tier and into the database. Every request you make here is one trip across your network.</p>
    <form @submit.prevent="add">
      <input v-model="text" placeholder="Write a note" maxlength="200" autofocus />
      <button type="submit">Add</button>
    </form>
    <p v-if="error" class="error">{{ error }}</p>
    <ul>
      <li v-for="n in notes" :key="n.id">
        <span class="id">#{{ n.id }}</span>
        <span class="text">{{ n.text }}</span>
        <span class="when">{{ n.created_at.replace('T', ' ').slice(0, 19) }}</span>
        <button class="del" @click="remove(n.id)" title="Delete">×</button>
      </li>
    </ul>
    <p v-if="!error && notes.length === 0" class="sub">No notes yet.</p>
    <footer v-if="version">backend {{ version.version }} · {{ version.commit }} · {{ version.hostname }}</footer>
  </main>
</template>

<style>
body { margin: 0; font-family: system-ui, sans-serif; background: #f4f4f8; color: #222; }
main { max-width: 640px; margin: 6vh auto; padding: 0 24px; }
h1 { color: #2b2557; border-left: 10px solid #e4067e; padding-left: 14px; }
.sub { color: #666; }
form { display: flex; gap: 8px; margin: 20px 0; }
input { flex: 1; font-size: 17px; padding: 10px 12px; border: 2px solid #2b2557; border-radius: 6px; }
input:focus { outline: none; border-color: #e4067e; }
button { font-size: 16px; padding: 10px 18px; background: #2b2557; color: #fff; border: 0; border-radius: 6px; cursor: pointer; }
button:hover { background: #e4067e; }
ul { list-style: none; padding: 0; margin: 0; }
li { display: flex; align-items: center; gap: 12px; padding: 10px 12px; background: #fff; border-radius: 6px; margin-bottom: 8px; }
.id { color: #999; font-family: ui-monospace, monospace; font-size: 13px; }
.text { flex: 1; }
.when { color: #999; font-size: 13px; }
.del { background: none; color: #999; font-size: 20px; padding: 0 6px; }
.del:hover { color: #e4067e; background: none; }
.error { color: #c0182b; }
footer { margin-top: 32px; color: #999; font-size: 13px; font-family: ui-monospace, monospace; }
</style>
