import { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { board, type BoardData } from '../api'

const REFRESH_MS = 5000

function Board() {
  const { id } = useParams()
  const sessionId = Number(id)

  const [data, setData] = useState<BoardData | null>(null)
  const [failed, setFailed] = useState(false)
  const [notFound, setNotFound] = useState(false)

  const alive = useRef(true)

  useEffect(() => {
    if (Number.isNaN(sessionId)) {
      setNotFound(true)
      return
    }

    alive.current = true

    async function tick() {
      try {
        const next = await board(sessionId)
        if (!alive.current) return
        setData(next)
        setFailed(false)
      } catch {
        if (!alive.current) return
        setFailed(true)
      }
    }

    tick()
    const timer = setInterval(tick, REFRESH_MS)

    return () => {
      alive.current = false
      clearInterval(timer)
    }
  }, [sessionId])

  if (notFound) {
    return <Screen><p className="text-4xl text-neutral-400">Unknown board</p></Screen>
  }

  if (!data) {
    return <Screen><p className="text-4xl text-neutral-400">Loading…</p></Screen>
  }

  return (
    <Screen>
      <div className="text-center">
        <p className="text-3xl font-medium text-neutral-400">{data.doctor_name}</p>

        <p className="mt-10 text-2xl uppercase tracking-[0.3em] text-neutral-500">
          Now serving
        </p>
        {data.now_serving === null ? (
          <p className="py-14 text-5xl font-medium text-neutral-600">
            Nobody in consultation
          </p>
        ) : (
          <p className="text-[14rem] font-bold leading-none tabular-nums">
            {data.now_serving}
          </p>
        )}

        {data.next.length > 0 && (
          <div className="mt-12">
            <p className="text-xl uppercase tracking-[0.3em] text-neutral-500">Next</p>
            <div className="mt-3 flex items-center justify-center gap-8 text-6xl font-semibold tabular-nums text-neutral-300">
              {data.next.map((token, i) => (
                <span key={token} className="flex items-center gap-8">
                  {i > 0 && (
                    <span aria-hidden className="text-4xl text-neutral-600">
                      &larr;
                    </span>
                  )}
                  {token}
                </span>
              ))}
            </div>
          </div>
        )}

        {data.delay_min > 0 && data.status === 'open' && (
          <p className="mt-12 text-3xl text-amber-400">
            Running about {data.delay_min} minutes late
          </p>
        )}

        {data.status !== 'open' && (
          <p className="mt-12 text-3xl text-neutral-400">This session is {data.status}</p>
        )}

        {failed && (
          <p className="mt-10 text-lg text-neutral-600">
            Reconnecting… showing the last known numbers
          </p>
        )}
      </div>
    </Screen>
  )
}

function Screen({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-svh items-center justify-center bg-neutral-950 p-10 text-neutral-50">
      {children}
    </div>
  )
}

export default Board
