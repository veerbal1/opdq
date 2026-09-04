import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'
import { logout } from '../api'

function Console() {
  const { session, setSession } = useAuth()
  const navigate = useNavigate()

  async function handleLogout() {
    await logout()
    setSession(null)
    navigate('/login', { replace: true })
  }

  return (
    <div>
      <header>
        Signed in as <strong>{session?.name}</strong> · clinic {session?.clinic_id} · {session?.role}
        <button onClick={handleLogout}>Log out</button>
      </header>

      <h1>Console</h1>
    </div>
  )
}

export default Console