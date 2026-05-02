import { useEffect, useMemo, useState } from 'react'
import ScenarioSelector from './components/ScenarioSelector'
import MessageList from './components/MessageList'
import MessageInput from './components/MessageInput'
import FeedbackPanel from './components/FeedbackPanel'

const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api'

const fallbackScenarios = [
  {
    id: 'conflict-colleague',
    title: 'Коллега сопротивляется идее',
    skill: 'Аргументация и деэскалация',
    difficulty: 'Средний',
    description: 'Сложный рабочий разговор с сопротивлением, рисками и поиском следующего шага.',
    goals: ['признать эмоцию', 'дать конкретику', 'закрепить шаг'],
  },
]

function App() {
  const [view, setView] = useState('home')
  const [user, setUser] = useState(() => {
    const saved = localStorage.getItem('softskills:user')
    return saved ? JSON.parse(saved) : null
  })
  const [sessions, setSessions] = useState([])
  const [sessionId, setSessionId] = useState('')
  const [scenario, setScenario] = useState(null)
  const [scenarios, setScenarios] = useState(fallbackScenarios)
  const [messages, setMessages] = useState([])
  const [feedback, setFeedback] = useState(null)
  const [loading, setLoading] = useState(false)
  const [aiTyping, setAiTyping] = useState(false)
  const [error, setError] = useState('')
  const [modelState, setModelState] = useState('Проверяем модель...')
  const [authMode, setAuthMode] = useState('login')
  const [authForm, setAuthForm] = useState({ name: '', email: '', password: '' })

  const authHeaders = useMemo(() => (
    user ? { 'X-User-ID': user.id } : {}
  ), [user])

  useEffect(() => {
    let stopped = false
    let attempts = 0

    const tick = async () => {
      attempts += 1
      const ready = await loadBootData()
      if (!stopped && !ready && attempts < 40) {
        setTimeout(tick, 5000)
      }
    }

    tick()
    return () => {
      stopped = true
    }
  }, [])

  useEffect(() => {
    if (user) {
      localStorage.setItem('softskills:user', JSON.stringify(user))
      loadProfile(user.id)
    } else {
      localStorage.removeItem('softskills:user')
      setSessions([])
    }
  }, [user?.id])

  const progress = useMemo(() => {
    const userTurns = messages.filter((message) => message.role === 'user').length
    return Math.min(100, userTurns * 22)
  }, [messages])

  const remaining = user ? Math.max(0, user.daily_limit - user.used_today) : 0

  async function loadBootData() {
    try {
      const [scenarioResponse, modelResponse] = await Promise.all([
        fetch(`${apiUrl}/scenarios`),
        fetch(`${apiUrl}/check-models`),
      ])

      if (scenarioResponse.ok) {
        const data = await scenarioResponse.json()
        if (Array.isArray(data.scenarios) && data.scenarios.length) {
          setScenarios(data.scenarios)
        }
      }

      setModelState(modelResponse.ok ? 'Сервис online' : 'Модель загружается...')
      return modelResponse.ok
    } catch {
      setModelState('Модель загружается...')
      return false
    }
  }

  async function loadProfile(userId = user?.id) {
    if (!userId) return
    try {
      const response = await fetch(`${apiUrl}/profile`, {
        headers: { 'X-User-ID': userId },
      })
      const data = await response.json()
      if (response.ok) {
        setUser(data.user)
        setSessions(Array.isArray(data.sessions) ? data.sessions : [])
      }
    } catch {
      // Profile refresh is non-blocking for the interface.
    }
  }

  async function submitAuth(event) {
    event.preventDefault()
    setLoading(true)
    setError('')
    try {
      const response = await fetch(`${apiUrl}/${authMode === 'login' ? 'login' : 'register'}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(authForm),
      })
      const data = await response.json()
      if (!response.ok) {
        setError(data.error || 'Не удалось войти')
        return
      }
      setUser(data.user)
      setView('trainer')
    } catch (err) {
      setError(err.message || 'Ошибка сети')
    } finally {
      setLoading(false)
    }
  }

  async function startSession(selectedScenario) {
    if (!user) {
      setView('account')
      setError('Сначала войдите или зарегистрируйтесь.')
      return
    }

    setLoading(true)
    setError('')
    setFeedback(null)
    try {
      const response = await fetch(`${apiUrl}/start-session`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders },
        body: JSON.stringify({ scenario: selectedScenario?.id || 'random' }),
      })

      const data = await response.json()
      if (!response.ok) {
        setError(data.error || 'Не удалось начать сессию. Проверьте backend и inference-сервис.')
        await loadProfile()
        return
      }

      setSessionId(data.session_id)
      setScenario(data.scenario_data || selectedScenario)
      setMessages(data.initial_message ? [{ role: 'assistant', text: data.initial_message }] : [])
      setView('trainer')
      await loadProfile()
    } catch (err) {
      setError(err.message || 'Ошибка сети при запуске сессии')
    } finally {
      setLoading(false)
    }
  }

  async function sendMessage(text) {
    if (!sessionId || !text) return
    setMessages((prev) => [...prev, { role: 'user', text }])
    setAiTyping(true)
    setError('')

    try {
      const response = await fetch(`${apiUrl}/send-message`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders },
        body: JSON.stringify({ session_id: sessionId, message: text }),
      })

      const data = await response.json()
      if (!response.ok) {
        setError(data.error || 'Сервер не вернул ответ от модели.')
        return
      }

      if (data.reply) {
        setMessages((prev) => [...prev, { role: 'assistant', text: data.reply }])
      } else {
        setError('Сервер вернул пустой ответ от inference-модели.')
      }
    } catch (err) {
      setError(err.message || 'Ошибка сети при отправке сообщения')
    } finally {
      setAiTyping(false)
    }
  }

  async function finishSession() {
    if (!sessionId) return
    setLoading(true)
    setError('')
    try {
      const response = await fetch(`${apiUrl}/get-feedback?session_id=${sessionId}`, {
        headers: authHeaders,
      })
      const data = await response.json()
      if (!response.ok) {
        setError(data.error || 'Не удалось получить разбор диалога.')
        return
      }
      setFeedback(data)
      await loadProfile()
    } catch (err) {
      setError(err.message || 'Ошибка сети при получении разбора')
    } finally {
      setLoading(false)
    }
  }

  async function openSession(id) {
    if (!user) return
    setLoading(true)
    setError('')
    try {
      const response = await fetch(`${apiUrl}/sessions/${id}`, { headers: authHeaders })
      const data = await response.json()
      if (!response.ok) {
        setError(data.error || 'Не удалось открыть диалог')
        return
      }
      setSessionId(data.id)
      setScenario({
        id: data.scenario,
        title: data.scenario_title,
        skill: 'История',
        description: 'Сохраненный диалог из личного кабинета',
        goals: [],
      })
      setMessages(data.messages || [])
      setFeedback(null)
      setView('trainer')
    } catch (err) {
      setError(err.message || 'Ошибка сети')
    } finally {
      setLoading(false)
    }
  }

  function logout() {
    setUser(null)
    setSessionId('')
    setScenario(null)
    setMessages([])
    setFeedback(null)
    setView('home')
  }

  return (
    <div className="app-shell">
      <nav className="top-nav">
        <button className="brand-button" onClick={() => setView('home')}>SoftSkill AI</button>
        <div className="nav-links">
          <button className={view === 'home' ? 'active' : ''} onClick={() => setView('home')}>Главная</button>
          <button className={view === 'trainer' ? 'active' : ''} onClick={() => setView('trainer')}>Тренировки</button>
          <button className={view === 'account' ? 'active' : ''} onClick={() => setView('account')}>Кабинет</button>
        </div>
        <div className="model-pill">{modelState}</div>
      </nav>

      <main className="workspace">
        {error && <div className="error-banner">{error}</div>}

        {view === 'home' && (
          <HomePage
            user={user}
            remaining={remaining}
            onStartRandom={() => startSession({ id: 'random' })}
            onOpenAccount={() => setView('account')}
            onOpenTrainer={() => setView('trainer')}
          />
        )}

        {view === 'trainer' && (
          !sessionId ? (
            <ScenarioSelector
              scenarios={scenarios}
              onSelect={startSession}
              onRandom={() => startSession({ id: 'random' })}
              loading={loading || !user || remaining <= 0}
              user={user}
              remaining={remaining}
            />
          ) : (
            <div className="training-grid">
              <section className="chat-panel">
                <div className="session-bar">
                  <div>
                    <p className="eyebrow">{scenario?.skill}</p>
                    <h2>{scenario?.title}</h2>
                  </div>
                  <div className="session-actions">
                    <button className="ghost-button" onClick={() => setSessionId('')} disabled={loading || aiTyping}>
                      Меню диалогов
                    </button>
                    <button className="finish-button" onClick={finishSession} disabled={loading || aiTyping}>
                      Разобрать диалог
                    </button>
                  </div>
                </div>

                <div className="progress-track" aria-label="Прогресс тренировки">
                  <span style={{ width: `${progress}%` }} />
                </div>

                <MessageList messages={messages} aiTyping={aiTyping} />
                <MessageInput onSend={sendMessage} disabled={loading || aiTyping} />
              </section>

              <FeedbackPanel feedback={feedback} scenario={scenario} progress={progress} />
            </div>
          )
        )}

        {view === 'account' && (
          <AccountPage
            user={user}
            sessions={sessions}
            authMode={authMode}
            setAuthMode={setAuthMode}
            authForm={authForm}
            setAuthForm={setAuthForm}
            onSubmitAuth={submitAuth}
            onLogout={logout}
            onOpenSession={openSession}
            loading={loading}
          />
        )}
      </main>
    </div>
  )
}

function HomePage({ user, remaining, onStartRandom, onOpenAccount, onOpenTrainer }) {
  return (
    <section className="home-layout">
      <div className="home-copy">
        <p className="eyebrow">AI roleplay trainer</p>
        <h1>Тренируйте сложные разговоры до реальной встречи.</h1>
        <p>
          Сервис создает напряженные рабочие диалоги, ведет роль собеседника через ml-service
          и после тренировки разбирает ваши ответы по эмпатии, ясности и конкретике.
        </p>
        <div className="hero-actions">
          <button className="finish-button" onClick={user ? onStartRandom : onOpenAccount}>
            {user ? 'Случайный диалог' : 'Войти и начать'}
          </button>
          <button className="ghost-button" onClick={onOpenTrainer}>Выбрать сценарий</button>
        </div>
      </div>
      <div className="home-panel">
        <span>Сегодня доступно</span>
        <strong>{user ? remaining : '5'}</strong>
        <p>{user ? 'тренировок до дневного сброса' : 'тренировок после регистрации'}</p>
      </div>
    </section>
  )
}

function AccountPage({
  user,
  sessions,
  authMode,
  setAuthMode,
  authForm,
  setAuthForm,
  onSubmitAuth,
  onLogout,
  onOpenSession,
  loading,
}) {
  if (!user) {
    return (
      <section className="account-layout">
        <div>
          <p className="eyebrow">Аккаунт</p>
          <h1>{authMode === 'login' ? 'Вход' : 'Регистрация'}</h1>
          <p className="muted">Кабинет нужен для дневных лимитов и истории диалогов.</p>
        </div>
        <form className="auth-card" onSubmit={onSubmitAuth}>
          {authMode === 'register' && (
            <label>
              Имя
              <input value={authForm.name} onChange={(event) => setAuthForm({ ...authForm, name: event.target.value })} />
            </label>
          )}
          <label>
            Email
            <input type="email" value={authForm.email} onChange={(event) => setAuthForm({ ...authForm, email: event.target.value })} />
          </label>
          <label>
            Пароль
            <input type="password" value={authForm.password} onChange={(event) => setAuthForm({ ...authForm, password: event.target.value })} />
          </label>
          <button className="finish-button" disabled={loading}>
            {authMode === 'login' ? 'Войти' : 'Создать аккаунт'}
          </button>
          <button type="button" className="ghost-button" onClick={() => setAuthMode(authMode === 'login' ? 'register' : 'login')}>
            {authMode === 'login' ? 'Нужна регистрация' : 'Уже есть аккаунт'}
          </button>
        </form>
      </section>
    )
  }

  const remaining = Math.max(0, user.daily_limit - user.used_today)

  return (
    <section className="account-layout">
      <div className="profile-card">
        <p className="eyebrow">Личный кабинет</p>
        <h1>{user.name}</h1>
        <p className="muted">{user.email}</p>
        <div className="limit-row">
          <span>Осталось сегодня</span>
          <strong>{remaining} / {user.daily_limit}</strong>
        </div>
        <button className="ghost-button" onClick={onLogout}>Выйти</button>
      </div>

      <div className="history-panel">
        <h2>Сохраненные диалоги</h2>
        {sessions.length ? (
          <div className="history-list">
            {sessions.map((session) => (
              <button key={session.id} className="history-item" onClick={() => onOpenSession(session.id)}>
                <strong>{session.scenario_title}</strong>
                <span>{session.message_count} сообщений</span>
              </button>
            ))}
          </div>
        ) : (
          <p className="muted">История появится после первой тренировки.</p>
        )}
      </div>
    </section>
  )
}

export default App
