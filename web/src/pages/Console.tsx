import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'
import {
  ApiError,
  closeSession,
  createWalkIn,
  listSessions,
  logout,
  queue,
  setDelay,
  transition,
  type QueueItem,
  type SessionItem,
} from '../api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function time(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function Console() {
  const { session, setSession } = useAuth()
  const navigate = useNavigate()

  const [sessions, setSessions] = useState<SessionItem[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [items, setItems] = useState<QueueItem[]>([])
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const [name, setName] = useState('')
  const [contact, setContact] = useState('')
  const [emergency, setEmergency] = useState(false)
  const [busy, setBusy] = useState(false)
  const [lastQR, setLastQR] = useState<{ token: number; publicID: string } | null>(null)

  const [delayInput, setDelayInput] = useState('')

  const selected = sessions.find((s) => s.id === selectedId) ?? null

  const loadSessions = useCallback(async () => {
    try {
      const list = await listSessions()
      setSessions(list)
      setSelectedId((current) =>
        current !== null && list.some((s) => s.id === current)
          ? current
          : (list[0]?.id ?? null),
      )
    } catch (e) {
      setError((e as Error).message)
    }
  }, [])

  const refreshQueue = useCallback(async (id: number) => {
    try {
      setItems(await queue(id))
    } catch (e) {
      setError((e as Error).message)
    }
  }, [])

  useEffect(() => {
    loadSessions()
  }, [loadSessions])

  useEffect(() => {
    if (selectedId !== null) refreshQueue(selectedId)
  }, [selectedId, refreshQueue])

  // Every mutation ends the same way: report, then refetch. Never patch local
  // state to guess what the server did — this screen's job is to be correct.
  async function run(fn: () => Promise<void>) {
    setError(null)
    setNotice(null)
    try {
      await fn()
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setError('Queue moved — reloading.')
        await loadSessions()
        if (selectedId !== null) await refreshQueue(selectedId)
        return
      }
      setError((e as Error).message)
    }
  }

  async function handleAdd(e: FormEvent) {
    e.preventDefault()
    if (selectedId === null) return
    setBusy(true)
    await run(async () => {
      const created = await createWalkIn(selectedId, name, contact, emergency ? 1 : 0)
      setName('')
      setContact('')
      setEmergency(false)
      setLastQR({ token: created.token_no, publicID: created.public_id })
      setNotice(`Token ${created.token_no} issued`)
      await refreshQueue(selectedId)
    })
    setBusy(false)
  }

  function move(appointmentId: number, to: string) {
    return run(async () => {
      await transition(appointmentId, to)
      if (selectedId !== null) await refreshQueue(selectedId)
    })
  }

  function applyDelay(e: FormEvent) {
    e.preventDefault()
    if (!selected) return
    const minutes = Number(delayInput)
    if (Number.isNaN(minutes) || minutes < 0) {
      setError('Delay must be a number of minutes.')
      return
    }
    return run(async () => {
      await setDelay(selected.id, minutes, selected.version)
      setDelayInput('')
      setNotice(`Delay set to ${minutes} minutes`)
      await loadSessions()
    })
  }

  function close() {
    if (!selected) return
    return run(async () => {
      await closeSession(selected.id, selected.version)
      setNotice('Session closed')
      await loadSessions()
    })
  }

  async function handleLogout() {
    await logout()
    setSession(null)
    navigate('/login', { replace: true })
  }

  const waiting = items.filter((a) => a.state === 'waiting').length

  return (
    <div className="min-h-svh bg-muted/40">
      <header className="flex items-center justify-between border-b bg-background px-6 py-3">
        <div className="flex items-baseline gap-2">
          <span className="font-semibold">OPD Queue</span>
          <span className="text-sm text-muted-foreground">
            {session?.name} · clinic {session?.clinic_id}
          </span>
        </div>
        <Button variant="outline" size="sm" onClick={handleLogout}>
          Log out
        </Button>
      </header>

      <main className="mx-auto grid max-w-4xl gap-4 p-6">
        {sessions.length === 0 ? (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No sessions today.
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle className="text-base">Session</CardTitle>
              {selected && (
                <Badge variant={selected.status === 'open' ? 'default' : 'secondary'}>
                  {selected.status}
                </Badge>
              )}
            </CardHeader>
            <CardContent className="grid gap-4">
              <select
                className="h-9 rounded-md border bg-background px-3 text-sm"
                value={selectedId ?? ''}
                onChange={(e) => setSelectedId(Number(e.target.value))}
              >
                {sessions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.doctor_name} · {time(s.starts_at)}–{time(s.ends_at)} · {s.status}
                  </option>
                ))}
              </select>

              {selected && (
                <div className="flex flex-wrap items-end gap-3">
                  <form onSubmit={applyDelay} className="flex items-end gap-2">
                    <div className="grid gap-1.5">
                      <Label htmlFor="delay" className="text-xs">
                        Delay (currently {selected.delay_min} min)
                      </Label>
                      <Input
                        id="delay"
                        className="w-28"
                        inputMode="numeric"
                        placeholder="minutes"
                        value={delayInput}
                        onChange={(e) => setDelayInput(e.target.value)}
                      />
                    </div>
                    <Button type="submit" variant="secondary" size="sm">
                      Set delay
                    </Button>
                  </form>

                  <Button
                    variant="outline"
                    size="sm"
                    onClick={close}
                    disabled={selected.status !== 'open'}
                  >
                    Close session
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {selected && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Add walk-in</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleAdd} className="flex flex-wrap items-end gap-3">
                <div className="grid gap-1.5">
                  <Label htmlFor="pname" className="text-xs">
                    Patient name
                  </Label>
                  <Input
                    id="pname"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="contact" className="text-xs">
                    Phone (optional)
                  </Label>
                  <Input
                    id="contact"
                    value={contact}
                    onChange={(e) => setContact(e.target.value)}
                  />
                </div>
                <label className="flex items-center gap-2 pb-2 text-sm">
                  <input
                    type="checkbox"
                    checked={emergency}
                    onChange={(e) => setEmergency(e.target.checked)}
                  />
                  Emergency
                </label>
                <Button type="submit" disabled={busy}>
                  {busy ? 'Adding…' : 'Add'}
                </Button>
              </form>

              {lastQR && (
                <div className="mt-4 flex items-center gap-4 rounded-md border p-3">
                  <img
                    src={`/api/q/${lastQR.publicID}/qr.png`}
                    alt={`QR for token ${lastQR.token}`}
                    className="size-24"
                  />
                  <div className="text-sm">
                    <p className="font-medium">Token {lastQR.token}</p>
                    <p className="text-muted-foreground">
                      Ask the patient to scan this for live status.
                    </p>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {selected && (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Queue · {waiting} waiting</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-20">Token</TableHead>
                    <TableHead>Patient</TableHead>
                    <TableHead className="w-32">Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-muted-foreground">
                        Nobody in the queue.
                      </TableCell>
                    </TableRow>
                  )}
                  {items.map((a) => (
                    <TableRow
                      key={a.id}
                      className={
                        a.state === 'in_consultation'
                          ? 'bg-accent/50'
                          : a.state === 'absent'
                            ? 'opacity-60'
                            : undefined
                      }
                    >
                      <TableCell className="font-medium">{a.token_no}</TableCell>
                      <TableCell>
                        {a.patient_name}
                        {a.priority === 1 && (
                          <Badge variant="destructive" className="ml-2">
                            emergency
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {a.state === 'in_consultation'
                          ? 'with doctor'
                          : a.state === 'absent'
                            ? 'no-show'
                            : 'waiting'}
                      </TableCell>
                      <TableCell className="space-x-2 text-right">
                        {a.state === 'absent' ? (
                          // Re-check-in sends them back to waiting. The server
                          // resets queued_at, so they land at the back of the
                          // line — not where they were before.
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => move(a.id, 'waiting')}
                          >
                            Re-check-in
                          </Button>
                        ) : (
                          <>
                            {a.state === 'waiting' ? (
                              <Button size="sm" onClick={() => move(a.id, 'in_consultation')}>
                                Call
                              </Button>
                            ) : (
                              <Button size="sm" onClick={() => move(a.id, 'done')}>
                                Done
                              </Button>
                            )}
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => move(a.id, 'absent')}
                            >
                              No-show
                            </Button>
                          </>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}

        {notice && <p className="text-sm text-muted-foreground">{notice}</p>}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
      </main>
    </div>
  )
}

export default Console
