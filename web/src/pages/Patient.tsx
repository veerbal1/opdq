import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { patient, type PatientData } from '../api'

const REFRESH_MS = 10000

function minutesUntil(iso: string) {
  const diff = new Date(iso).getTime() - Date.now()
  return Math.max(0, Math.round(diff / 60000))
}

// Public page, opened from a QR code. Whoever holds the link is the patient —
// there is no account and nothing to log into.
function Patient() {
  const { publicId } = useParams()

  const [data, setData] = useState<PatientData | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [stale, setStale] = useState(false)
  const alive = useRef(true)

  useEffect(() => {
    if (!publicId) {
      setNotFound(true)
      return
    }
    alive.current = true

    async function tick() {
      try {
        const next = await patient(publicId!)
        if (!alive.current) return
        setData(next)
        setStale(false)
      } catch {
        if (!alive.current) return
        // Only claim "not found" if we never had anything; otherwise this is a
        // dropped request on a phone, and the last good answer is still useful.
        setData((current) => {
          if (current === null) setNotFound(true)
          else setStale(true)
          return current
        })
      }
    }

    tick()
    const timer = setInterval(tick, REFRESH_MS)
    return () => {
      alive.current = false
      clearInterval(timer)
    }
  }, [publicId])

  if (notFound) {
    return (
      <Screen>
        <h1 className="text-2xl font-semibold">Not found</h1>
        <p className="mt-2 text-muted-foreground">
          This link is not valid. Please check with reception.
        </p>
      </Screen>
    )
  }

  if (!data) {
    return (
      <Screen>
        <p className="text-muted-foreground">Loading…</p>
      </Screen>
    )
  }

  return (
    <Screen>
      <p className="text-sm uppercase tracking-widest text-muted-foreground">Your token</p>
      <p className="text-8xl font-bold tabular-nums">{data.token_no}</p>

      {data.state === 'waiting' && (
        <>
          <p className="mt-8 text-lg">
            {data.position === 0
              ? 'You are next.'
              : `${data.position} ${data.position === 1 ? 'person' : 'people'} ahead of you.`}
          </p>
          {data.eta && (
            <p className="mt-1 text-muted-foreground">
              About {minutesUntil(data.eta)} minutes.
            </p>
          )}
        </>
      )}

      {data.state === 'in_consultation' && (
        <p className="mt-8 text-lg font-medium">You are with the doctor.</p>
      )}

      {data.state === 'done' && (
        <p className="mt-8 text-lg font-medium">Visit complete.</p>
      )}

      {data.state === 'absent' && (
        <p className="mt-8 text-lg font-medium text-amber-600">
          You were marked absent. Please see reception.
        </p>
      )}

      {data.now_serving !== null && (
        <p className="mt-10 text-sm text-muted-foreground">
          Now serving token {data.now_serving}
        </p>
      )}

      {stale && (
        <p className="mt-6 text-xs text-muted-foreground">
          Reconnecting… showing the last known status
        </p>
      )}
    </Screen>
  )
}

function Screen({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center bg-muted/40 p-8 text-center">
      {children}
    </div>
  )
}

export default Patient
