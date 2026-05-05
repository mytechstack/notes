import { useState, type MouseEvent } from 'react'
import type { Policy } from '../types/policy'

interface Props {
  policies: Policy[]
  loading: boolean
  selected: Policy | null
  onSelect: (p: Policy) => void
  onNew: () => void
  onDelete: (p: Policy) => void
}

export default function PolicyList({ policies, loading, selected, onSelect, onNew, onDelete }: Props) {
  const [search, setSearch] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const filtered = policies.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase()) ||
    p.path.toLowerCase().includes(search.toLowerCase())
  )

  const activeCount = policies.filter(p => p.active).length

  const handleDelete = (e: MouseEvent<HTMLButtonElement>, policy: Policy) => {
    e.stopPropagation()
    if (confirmDelete === policy.id) {
      onDelete(policy)
      setConfirmDelete(null)
    } else {
      setConfirmDelete(String(policy.id))
      setTimeout(() => setConfirmDelete(null), 3000)
    }
  }

  return (
    <aside className="w-72 shrink-0 flex flex-col bg-slate-900 border-r border-slate-800 overflow-hidden">
      {/* Sidebar header */}
      <div className="px-4 pt-4 pb-3 border-b border-slate-800">
        <div className="flex items-center justify-between mb-3">
          <div>
            <span className="text-xs font-semibold uppercase tracking-widest text-slate-400">Policies</span>
            <span className="ml-2 text-xs text-slate-600">{activeCount}/{policies.length} active</span>
          </div>
          <button
            onClick={onNew}
            title="New policy"
            className="flex items-center justify-center w-7 h-7 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white transition-colors"
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
          </button>
        </div>
        {/* Search */}
        <div className="relative">
          <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-500 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
          </svg>
          <input
            type="text"
            placeholder="Filter policies…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full bg-slate-800 border border-slate-700 rounded-md pl-8 pr-3 py-1.5 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 focus:border-indigo-500 transition-colors"
          />
        </div>
      </div>

      {/* Policy list */}
      <div className="flex-1 overflow-y-auto py-1">
        {loading ? (
          <div className="space-y-1 p-2">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-14 bg-slate-800 rounded-lg animate-pulse" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 text-center px-4">
            {search ? (
              <>
                <p className="text-slate-500 text-sm">No policies match</p>
                <button onClick={() => setSearch('')} className="text-indigo-400 text-xs mt-1 hover:underline">Clear filter</button>
              </>
            ) : (
              <>
                <p className="text-slate-500 text-sm">No policies yet</p>
                <button onClick={onNew} className="text-indigo-400 text-xs mt-1 hover:underline">Create one</button>
              </>
            )}
          </div>
        ) : (
          <ul className="px-2 space-y-0.5">
            {filtered.map(policy => {
              const isSelected = selected?.id === policy.id
              const needsConfirm = confirmDelete === String(policy.id)
              return (
                <li key={policy.id}>
                  <button
                    onClick={() => onSelect(policy)}
                    className={`group w-full flex items-start gap-2.5 px-3 py-2.5 rounded-lg text-left transition-colors
                      ${isSelected
                        ? 'bg-indigo-600/20 border border-indigo-500/30'
                        : 'hover:bg-slate-800 border border-transparent'
                      }`}
                  >
                    {/* Active indicator */}
                    <span className={`mt-1.5 w-1.5 h-1.5 rounded-full shrink-0 ${policy.active ? 'bg-emerald-400' : 'bg-slate-600'}`} />

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className={`text-sm font-medium truncate ${isSelected ? 'text-indigo-300' : 'text-slate-200'}`}>
                          {policy.name}
                        </span>
                        <span className="shrink-0 text-xs text-slate-600 font-mono">v{policy.version}</span>
                      </div>
                      <p className="text-xs text-slate-500 truncate mt-0.5 font-mono">{policy.path}</p>
                    </div>

                    {/* Delete button */}
                    <button
                      onClick={e => handleDelete(e, policy)}
                      title={needsConfirm ? 'Click again to confirm' : 'Delete policy'}
                      className={`shrink-0 opacity-0 group-hover:opacity-100 focus:opacity-100 mt-0.5 p-1 rounded transition-all
                        ${needsConfirm ? 'opacity-100 text-red-400 bg-red-900/40' : 'text-slate-500 hover:text-red-400 hover:bg-red-900/30'}`}
                    >
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
                      </svg>
                    </button>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </div>

      {/* Footer */}
      <div className="px-4 py-3 border-t border-slate-800">
        <button
          onClick={onNew}
          className="w-full flex items-center justify-center gap-2 py-2 rounded-lg border border-dashed border-slate-700 text-slate-500 hover:border-indigo-500 hover:text-indigo-400 text-xs transition-colors"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          New Policy
        </button>
      </div>
    </aside>
  )
}
