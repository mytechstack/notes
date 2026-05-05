import type { ToastItem } from '../App'

export default function Toast({ item }: { item: ToastItem }) {
  const isSuccess = item.type === 'success'
  return (
    <div className={`pointer-events-auto flex items-center gap-2.5 px-4 py-3 rounded-lg shadow-2xl text-sm font-medium border
      ${isSuccess
        ? 'bg-emerald-950 border-emerald-800 text-emerald-100'
        : 'bg-red-950 border-red-800 text-red-100'
      }`}
    >
      {isSuccess ? (
        <svg className="w-4 h-4 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
        </svg>
      ) : (
        <svg className="w-4 h-4 text-red-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
        </svg>
      )}
      {item.message}
    </div>
  )
}
