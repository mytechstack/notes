import { useEffect, useRef, useState, useCallback } from 'react'
import Editor, { type OnMount } from '@monaco-editor/react'
import type * as MonacoType from 'monaco-editor'
import axios from 'axios'
import type { Policy } from '../types/policy'

const API = 'http://localhost:8080'

interface Props {
  policy: Policy
  saving: boolean
  onSave: (p: Policy) => void
  onCancel: () => void
}

type OpaError = {
  code?: string
  message: string
  location?: { row: number; col: number; file?: string }
}

type ValidationStatus = 'idle' | 'checking' | 'valid' | 'invalid'

export default function PolicyEditor({ policy, saving, onSave, onCancel }: Props) {
  const [draft, setDraft] = useState<Policy>(policy)
  const [validation, setValidation] = useState<ValidationStatus>('idle')
  const [errors, setErrors] = useState<OpaError[]>([])

  const editorRef = useRef<MonacoType.editor.IStandaloneCodeEditor | null>(null)
  const monacoRef = useRef<typeof MonacoType | null>(null)

  useEffect(() => { setDraft(policy); setValidation('idle'); setErrors([]) }, [policy])

  // ── Monaco setup ────────────────────────────────────────────────────────────
  const handleMount: OnMount = useCallback((editor, monaco) => {
    editorRef.current = editor
    monacoRef.current = monaco

    monaco.languages.register({ id: 'rego' })
    monaco.languages.setMonarchTokensProvider('rego', {
      keywords: ['package', 'import', 'default', 'not', 'null', 'true', 'false',
                 'if', 'in', 'contains', 'every', 'some', 'as', 'with', 'else'],
      tokenizer: {
        root: [
          [/#.*$/, 'comment'],
          [/"([^"\\]|\\.)*"/, 'string'],
          [/`[^`]*`/, 'string'],
          [/\b(package|import|default|not|null|true|false|if|in|contains|every|some|as|with|else)\b/, 'keyword'],
          [/\b\d+(\.\d+)?\b/, 'number'],
        ],
      },
    } as MonacoType.languages.IMonarchLanguage)

    monaco.editor.defineTheme('opa-dark', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'keyword',    foreground: 'a78bfa', fontStyle: 'bold' },
        { token: 'comment',    foreground: '64748b', fontStyle: 'italic' },
        { token: 'string',     foreground: '86efac' },
        { token: 'number',     foreground: 'fb923c' },
      ],
      colors: {
        'editor.background':              '#0f172a',
        'editor.foreground':              '#e2e8f0',
        'editor.lineHighlightBackground': '#1e293b',
        'editorLineNumber.foreground':    '#334155',
        'editorLineNumber.activeForeground': '#64748b',
        'editor.selectionBackground':     '#312e81aa',
        'editorCursor.foreground':        '#818cf8',
        'editorGutter.background':        '#0f172a',
        'editorError.foreground':         '#f87171',
        'editorError.border':             '#f87171',
      },
    })
    monaco.editor.setTheme('opa-dark')
  }, [])

  // ── Validation ──────────────────────────────────────────────────────────────
  const applyMarkers = useCallback((errs: OpaError[]) => {
    const editor = editorRef.current
    const monaco = monacoRef.current
    if (!editor || !monaco) return
    const model = editor.getModel()
    if (!model) return
    monaco.editor.setModelMarkers(model, 'rego', errs.map(e => ({
      startLineNumber: e.location?.row ?? 1,
      startColumn:     e.location?.col ?? 1,
      endLineNumber:   e.location?.row ?? 1,
      endColumn:       model.getLineLength(e.location?.row ?? 1) + 1,
      message:         e.message,
      severity:        monaco.MarkerSeverity.Error,
    })))
  }, [])

  const clearMarkers = useCallback(() => {
    const editor = editorRef.current
    const monaco = monacoRef.current
    if (!editor || !monaco) return
    const model = editor.getModel()
    if (model) monaco.editor.setModelMarkers(model, 'rego', [])
  }, [])

  const validate = useCallback(async (content: string) => {
    if (!content.trim()) { setValidation('idle'); return }
    setValidation('checking')
    try {
      const { data } = await axios.post<{
        valid: boolean
        errors?: OpaError[]
      }>(`${API}/validate`, { content, path: draft.path || 'policy.rego' })

      if (data.valid) {
        setValidation('valid')
        setErrors([])
        clearMarkers()
      } else {
        const errs = data.errors ?? []
        setValidation('invalid')
        setErrors(errs)
        applyMarkers(errs)
      }
    } catch {
      setValidation('idle')
    }
  }, [draft.path, applyMarkers, clearMarkers])

  // Debounced auto-validate on content change
  useEffect(() => {
    setValidation('idle')
    clearMarkers()
    if (!draft.content.trim()) return
    const t = setTimeout(() => validate(draft.content), 1500)
    return () => clearTimeout(t)
  }, [draft.content]) // eslint-disable-line react-hooks/exhaustive-deps

  // ── Derived state ────────────────────────────────────────────────────────────
  const isNew       = !draft.id || draft.id === ''
  const hasChanges  = JSON.stringify(draft) !== JSON.stringify(policy)
  const canSave     = !saving && !!draft.name && !!draft.path && validation !== 'invalid'

  const goToError = (err: OpaError) => {
    if (!editorRef.current || !err.location) return
    editorRef.current.revealLineInCenter(err.location.row)
    editorRef.current.setPosition({ lineNumber: err.location.row, column: err.location.col })
    editorRef.current.focus()
  }

  return (
    <div className="h-full flex flex-col bg-slate-950">
      {/* ── Header ── */}
      <div className="shrink-0 flex items-center justify-between px-5 py-3 bg-slate-900 border-b border-slate-800">
        <div className="flex items-center gap-2 min-w-0">
          <svg className="w-4 h-4 text-slate-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
          </svg>
          <span className="text-sm font-medium text-slate-200 truncate">
            {isNew ? 'New Policy' : (draft.path || draft.name || 'Untitled')}
          </span>
          {!isNew && <span className="shrink-0 text-xs font-mono text-slate-600">v{draft.version}</span>}
          {hasChanges && !isNew && <span className="w-1.5 h-1.5 rounded-full bg-amber-400 shrink-0" title="Unsaved changes" />}
        </div>

        <div className="flex items-center gap-2">
          <ValidationBadge status={validation} errorCount={errors.length} />
          <span className={`flex items-center gap-1.5 text-xs px-2 py-1 rounded-full border
            ${draft.active
              ? 'text-emerald-400 border-emerald-800 bg-emerald-950'
              : 'text-slate-500 border-slate-700 bg-slate-800'}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${draft.active ? 'bg-emerald-400' : 'bg-slate-600'}`} />
            {draft.active ? 'Active' : 'Inactive'}
          </span>
        </div>
      </div>

      {/* ── Fields ── */}
      <div className="shrink-0 grid grid-cols-2 gap-4 px-5 py-4 bg-slate-900 border-b border-slate-800">
        <div>
          <label className="block text-xs font-medium text-slate-500 uppercase tracking-wide mb-1.5">Name</label>
          <input
            type="text"
            value={draft.name}
            onChange={e => setDraft({ ...draft, name: e.target.value })}
            placeholder="e.g. rbac"
            className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-100 placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-all"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-500 uppercase tracking-wide mb-1.5">Path</label>
          <input
            type="text"
            value={draft.path}
            onChange={e => setDraft({ ...draft, path: e.target.value })}
            placeholder="e.g. rbac/rbac.rego"
            className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-100 placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-all font-mono"
          />
        </div>
      </div>

      {/* ── Monaco ── */}
      <div className="flex-1 overflow-hidden min-h-0">
        <Editor
          height="100%"
          language="rego"
          theme="opa-dark"
          value={draft.content}
          onChange={val => setDraft({ ...draft, content: val ?? '' })}
          onMount={handleMount}
          options={{
            minimap:                    { enabled: false },
            fontSize:                   13,
            lineHeight:                 20,
            fontFamily:                 '"JetBrains Mono","Fira Code",Menlo,monospace',
            scrollBeyondLastLine:       false,
            padding:                    { top: 16, bottom: 16 },
            renderLineHighlight:        'line',
            smoothScrolling:            true,
            cursorBlinking:             'smooth',
            cursorSmoothCaretAnimation: 'on',
            wordWrap:                   'on',
          }}
        />
      </div>

      {/* ── Error panel ── */}
      {validation === 'invalid' && errors.length > 0 && (
        <div className="shrink-0 max-h-36 overflow-y-auto bg-red-950/40 border-t border-red-900/60">
          <div className="px-4 py-2 flex items-center gap-2 border-b border-red-900/40">
            <svg className="w-3.5 h-3.5 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
            </svg>
            <span className="text-xs font-semibold text-red-400">{errors.length} error{errors.length > 1 ? 's' : ''}</span>
          </div>
          <ul className="py-1">
            {errors.map((err, i) => (
              <li key={i}>
                <button
                  onClick={() => goToError(err)}
                  className="w-full flex items-start gap-3 px-4 py-1.5 text-left hover:bg-red-900/20 transition-colors"
                >
                  {err.location && (
                    <span className="shrink-0 font-mono text-xs text-red-500 pt-0.5">
                      {err.location.row}:{err.location.col}
                    </span>
                  )}
                  <span className="text-xs text-red-300">{err.message}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* ── Footer ── */}
      <div className="shrink-0 flex items-center justify-between px-5 py-3 bg-slate-900 border-t border-slate-800">
        <label className="flex items-center gap-3 cursor-pointer select-none">
          <button
            type="button"
            role="switch"
            aria-checked={draft.active}
            onClick={() => setDraft({ ...draft, active: !draft.active })}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-slate-900
              ${draft.active ? 'bg-indigo-600' : 'bg-slate-700'}`}
          >
            <span
              style={{ transform: draft.active ? 'translateX(18px)' : 'translateX(2px)' }}
              className="inline-block h-3.5 w-3.5 rounded-full bg-white shadow transition-transform duration-200"
            />
          </button>
          <span className="text-sm text-slate-400">
            {draft.active ? 'Active — included in bundle' : 'Inactive — excluded from bundle'}
          </span>
        </label>

        <div className="flex items-center gap-2">
          <button
            onClick={() => validate(draft.content)}
            disabled={validation === 'checking'}
            className="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-slate-300 hover:text-slate-100 border border-slate-700 hover:border-slate-600 rounded-lg transition-colors disabled:opacity-50"
          >
            {validation === 'checking' ? (
              <svg className="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            ) : (
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            )}
            Check
          </button>
          <button
            onClick={onCancel}
            disabled={saving}
            className="px-4 py-2 text-sm font-medium text-slate-400 hover:text-slate-200 border border-slate-700 hover:border-slate-600 rounded-lg transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            onClick={() => onSave(draft)}
            disabled={!canSave}
            title={validation === 'invalid' ? 'Fix errors before saving' : undefined}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? (
              <>
                <svg className="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                Saving…
              </>
            ) : isNew ? 'Create Policy' : 'Save Changes'}
          </button>
        </div>
      </div>
    </div>
  )
}

function ValidationBadge({ status, errorCount }: { status: ValidationStatus; errorCount: number }) {
  if (status === 'idle') return null
  if (status === 'checking') return (
    <span className="flex items-center gap-1.5 text-xs text-slate-400 px-2 py-1 rounded-full border border-slate-700 bg-slate-800">
      <svg className="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
      </svg>
      Checking…
    </span>
  )
  if (status === 'valid') return (
    <span className="flex items-center gap-1.5 text-xs text-emerald-400 px-2 py-1 rounded-full border border-emerald-800 bg-emerald-950">
      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
      </svg>
      Valid
    </span>
  )
  return (
    <span className="flex items-center gap-1.5 text-xs text-red-400 px-2 py-1 rounded-full border border-red-800 bg-red-950">
      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
      </svg>
      {errorCount} error{errorCount !== 1 ? 's' : ''}
    </span>
  )
}
