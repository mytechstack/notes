import { useState, useEffect, useCallback } from 'react'
import axios from 'axios'
import PolicyList from './components/PolicyList'
import PolicyEditor from './components/PolicyEditor'
import Toast from './components/Toast'
import type { Policy } from './types/policy'

const API = 'http://localhost:8080'

export type ToastItem = {
  id: number
  type: 'success' | 'error'
  message: string
}

const BLANK_POLICY: Policy = {
  id: '',
  name: '',
  path: '',
  content: `package mypackage

import rego.v1

default allow := false

allow if {
    # Add your rules here
}
`,
  active: true,
}

export default function App() {
  const [policies, setPolicies] = useState<Policy[]>([])
  const [selected, setSelected] = useState<Policy | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const toast = useCallback((type: 'success' | 'error', message: string) => {
    const id = Date.now()
    setToasts(prev => [...prev, { id, type, message }])
    setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 3500)
  }, [])

  const fetchPolicies = useCallback(async () => {
    try {
      const { data } = await axios.get<Policy[]>(`${API}/policies`)
      setPolicies(data ?? [])
    } catch {
      toast('error', 'Could not reach the bundle server')
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => { fetchPolicies() }, [fetchPolicies])

  const handleSave = async (policy: Policy) => {
    setSaving(true)
    try {
      if (policy.id && policy.id !== '') {
        await axios.put(`${API}/policies/${policy.id}`, policy)
        toast('success', 'Policy updated')
      } else {
        await axios.post(`${API}/policies`, policy)
        toast('success', 'Policy created')
      }
      await fetchPolicies()
      setSelected(null)
    } catch {
      toast('error', 'Failed to save policy')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (policy: Policy) => {
    try {
      await axios.delete(`${API}/policies/${policy.id}`)
      toast('success', 'Policy deleted')
      if (selected?.id === policy.id) setSelected(null)
      await fetchPolicies()
    } catch {
      toast('error', 'Failed to delete policy')
    }
  }

  return (
    <div className="h-screen flex flex-col bg-slate-950 text-slate-100 overflow-hidden">
      {/* ── Header ── */}
      <header className="shrink-0 flex items-center justify-between px-5 h-14 bg-slate-900 border-b border-slate-800">
        <div className="flex items-center gap-3">
          <div className="flex items-center justify-center w-7 h-7 rounded-md bg-indigo-600 text-xs font-bold tracking-tight">
            OPA
          </div>
          <span className="font-semibold text-sm text-slate-100">Policy Manager</span>
          <span className="hidden sm:inline text-slate-600 text-xs">/ Control Plane</span>
        </div>
        <div className="flex items-center gap-4 text-xs text-slate-500">
          <span className="hidden sm:inline">{policies.length} {policies.length === 1 ? 'policy' : 'policies'}</span>
          <div className="flex items-center gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            <span>bundle-server</span>
          </div>
        </div>
      </header>

      {/* ── Body ── */}
      <div className="flex flex-1 overflow-hidden">
        <PolicyList
          policies={policies}
          loading={loading}
          selected={selected}
          onSelect={setSelected}
          onNew={() => setSelected({ ...BLANK_POLICY })}
          onDelete={handleDelete}
        />

        <main className="flex-1 overflow-hidden">
          {selected ? (
            <PolicyEditor
              key={selected.id ?? 'new'}
              policy={selected}
              saving={saving}
              onSave={handleSave}
              onCancel={() => setSelected(null)}
            />
          ) : (
            <EmptyState onNew={() => setSelected({ ...BLANK_POLICY })} />
          )}
        </main>
      </div>

      {/* ── Toasts ── */}
      <div className="fixed bottom-5 right-5 flex flex-col gap-2 z-50 pointer-events-none">
        {toasts.map(t => <Toast key={t.id} item={t} />)}
      </div>
    </div>
  )
}

function EmptyState({ onNew }: { onNew: () => void }) {
  return (
    <div className="h-full flex flex-col items-center justify-center gap-4 text-center p-10">
      <div className="w-14 h-14 rounded-2xl bg-slate-800 border border-slate-700 flex items-center justify-center">
        <svg className="w-7 h-7 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
        </svg>
      </div>
      <div>
        <p className="text-slate-300 font-medium">No policy selected</p>
        <p className="text-slate-500 text-sm mt-1">Pick one from the list or create a new policy</p>
      </div>
      <button
        onClick={onNew}
        className="mt-1 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-sm font-medium rounded-lg transition-colors"
      >
        New Policy
      </button>
    </div>
  )
}
