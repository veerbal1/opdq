import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'
import {
  logout,
  listSessions,
  queue,
  createWalkIn,
  transition,
  type SessionItem,
  type QueueItem,
} from '../api'

function Console() {
  const { session, setSession } = useAuth()
  const navigate = useNavigate()

  const [sessions, setSessions] = useState<SessionItem[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [items, setItems] = useState<QueueItem[]>([])
  const [error, setError] = useState<string | null>(null)

  const [name, setName] = useState('')
  const [contact, setContact] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    listSessions()
      .then((list) => {
        setSessions(list)
        if (list.length > 0) setSelectedId(list[0].id)
      })
      .catch((e) => setError((e as Error).message))
  }, [])

  const refreshQueue = useCallback(async (id: number) => {
    try {
      setItems(await queue(id))
    } catch (e) {
      setError((e as Error).message)
    }
  }, [])

  useEffect(() => {
    if (selectedId !== null) refreshQueue(selectedId)
  }, [selectedId, refreshQueue])

  async function handleAdd(e: FormEvent) {
    e.preventDefault()
    if (selectedId === null) return
    setBusy(true)
    setError(null)
    try {
      const created = await createWalkIn(selectedId, name, contact)
      setName('')
      setContact('')
      await refreshQueue(selectedId)
      setError(`Token ${created.token_no} issued`)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  async function move(appointmentId: number, to: string) {
    setError(null)
    try {
      await transition(appointmentId, to)
      if (selectedId !== null) await refreshQueue(selectedId)
    } catch (err) {
      setError((err as Error).message)
    }
  }

  async function handleLogout() {
    await logout()
    setSession(null)
    navigate('/login', { replace: true })
  }

  const selected = sessions.find((s) => s.id === selectedId)

  return (
    <div style={{ padding: 16, fontFamily: 'system-ui', maxWidth: 720 }}>
      <header style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
        <span>
          Signed in as <strong>{session?.name}</strong> · clinic {session?.clinic_id}
        </span>
        <button onClick={handleLogout}>Log out</button>
      </header>

      <h1>Console</h1>

      {sessions.length === 0 ? (
        <p>No sessions today.</p>
      ) : (
        <label>
          Session{' '}
          <select
            value={selectedId ?? ''}
            onChange={(e) => setSelectedId(Number(e.target.value))}
          >
            {sessions.map((s) => (
              <option key={s.id} value={s.id}>
                {s.doctor_name} · {new Date(s.starts_at).toLocaleTimeString()} –{' '}
                {new Date(s.ends_at).toLocaleTimeString()} · {s.status}
              </option>
            ))}
          </select>
        </label>
      )}

      {selected && (
        <>
          <h2>Add walk-in</h2>
          <form onSubmit={handleAdd} style={{ display: 'flex', gap: 8 }}>
            <input
              placeholder="Patient name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
            <input
              placeholder="Contact (optional)"
              value={contact}
              onChange={(e) => setContact(e.target.value)}
            />
            <button type="submit" disabled={busy}>
              {busy ? 'Adding…' : 'Add'}
            </button>
          </form>

          <h2>Queue ({items.length} waiting)</h2>
          <table cellPadding={6}>
            <thead>
              <tr>
                <th align="left">Token</th>
                <th align="left">Patient</th>
                <th align="left">Priority</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((a) => (
                <tr key={a.id}>
                  <td>{a.token_no}</td>
                  <td>{a.patient_name}</td>
                  <td>{a.priority === 1 ? 'emergency' : '—'}</td>
                  <td>
                    <button onClick={() => move(a.id, 'in_consultation')}>Call</button>{' '}
                    <button onClick={() => move(a.id, 'absent')}>No-show</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {error && <p role="alert">{error}</p>}
    </div>
  )
}

export default Console
