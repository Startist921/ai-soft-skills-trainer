import { useEffect, useMemo, useState } from 'react'
import ScenarioSelector from './components/ScenarioSelector'
import MessageList from './components/MessageList'
import MessageInput from './components/MessageInput'
import FeedbackPanel from './components/FeedbackPanel'

const apiUrl = import.meta.env.VITE_API_URL || '/api'

function getNetworkErrorMessage(error, fallbackText) {
  if (error instanceof TypeError) {
    return 'Сетевой запрос не дошел до API. Проверьте URL backend/nginx и CORS.'
  }
  return error?.message || fallbackText
}

function getPasswordStrength(password) {
  if (!password) return { label: 'пустой', score: 0 }
  let score = 0
  if (password.length >= 8) score += 1
  if (/[A-Z]/.test(password)) score += 1
  if (/[a-z]/.test(password)) score += 1
  if (/[0-9]/.test(password)) score += 1
  if (/[^A-Za-z0-9]/.test(password)) score += 1
  if (score <= 2) return { label: 'слабый', score }
  if (score <= 4) return { label: 'средний', score }
  return { label: 'сильный', score }
}

function buildProfileProgress(user, sessions, stats) {
  const completed = Number(stats?.completed_sessions || (Array.isArray(sessions) ? sessions.length : 0))
  const avgScore = Number(stats?.average_score || 0)
  const avgPercent = Number(stats?.average_percent || Math.round(avgScore * 10))
  const trend = stats?.trend || 'stable'
  const doneRatio = user?.daily_limit ? Math.min(1, (user.used_today || 0) / user.daily_limit) : 0
  const trendLabel = trend === 'up' ? 'растет' : trend === 'down' ? 'проседает' : 'стабильный'

  if (completed >= 20 || avgScore >= 8) {
    return {
      stage: 'Продвинутый этап',
      level: 'Переговорщик',
      description: `Средняя оценка ${avgScore.toFixed(1)}/10 (${avgPercent}%). Тренд: ${trendLabel}. Вы уверенно ведете сложные разговоры.`,
      percent: Math.max(70, Math.min(100, avgPercent || Math.round(70 + doneRatio * 30))),
    }
  }
  if (completed >= 8 || avgScore >= 6) {
    return {
      stage: 'Устойчивый прогресс',
      level: 'Практик',
      description: `Средняя оценка ${avgScore.toFixed(1)}/10 (${avgPercent}%). Тренд: ${trendLabel}. Навык стабилизируется, ответы становятся точнее.`,
      percent: Math.max(40, Math.min(85, avgPercent || Math.round(40 + doneRatio * 40))),
    }
  }
  return {
    stage: 'Базовый этап',
    level: 'Старт',
    description: `Пока собрано мало данных. Средняя оценка ${avgScore.toFixed(1)}/10. Фокус: эмпатия, ясность и конкретный следующий шаг.`,
    percent: Math.max(10, Math.min(45, avgPercent || Math.round(10 + doneRatio * 30))),
  }
}

function buildSparklinePoints(scores, width, height, padding) {
  if (!Array.isArray(scores) || scores.length < 2) return ''
  const stepX = (width - padding * 2) / (scores.length - 1)
  const points = scores.map((score, idx) => {
    const x = padding + idx * stepX
    const y = padding + (10 - Math.max(0, Math.min(10, score))) * ((height - padding * 2) / 10)
    return `${x},${y}`
  })
  return points.join(' ')
}

function ProgressSparkline({ scores }) {
  if (!Array.isArray(scores) || scores.length < 2) {
    return <p className="muted">График появится после нескольких разборов.</p>
  }

  const normalized = [...scores].reverse()
  const width = 280
  const height = 90
  const padding = 10
  const points = buildSparklinePoints(normalized, width, height, padding)
  const last = normalized[normalized.length - 1]

  return (
    <div className="sparkline-wrap">
      <svg viewBox={`0 0 ${width} ${height}`} className="sparkline" role="img" aria-label="Динамика оценок">
        <polyline points={points} />
      </svg>
      <div className="sparkline-meta">
        <span>Последняя: {last}/10</span>
        <span>Точек: {normalized.length}</span>
      </div>
    </div>
  )
}

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
  const [profileStats, setProfileStats] = useState(null)
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
  const [authForm, setAuthForm] = useState({ name: '', email: '', password: '', confirmPassword: '' })

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
      setProfileStats(null)
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
        setProfileStats(data.stats || null)
      }
    } catch {
      // Profile refresh is non-blocking for the interface.
    }
  }

  async function submitAuth(event) {
    event.preventDefault()
    if (authMode === 'register') {
      const trimmedName = authForm.name.trim()
      if (!trimmedName) {
        setError('Введите имя для регистрации.')
        return
      }
      if (authForm.password.length < 6) {
        setError('Пароль должен быть не короче 6 символов.')
        return
      }
      if (authForm.password !== authForm.confirmPassword) {
        setError('Пароли не совпадают.')
        return
      }
    }
    setLoading(true)
    setError('')
    try {
      const response = await fetch(`${apiUrl}/${authMode === 'login' ? 'login' : 'register'}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: authForm.name.trim(),
          email: authForm.email.trim(),
          password: authForm.password,
        }),
      })
      const data = await response.json().catch(() => ({}))
      if (!response.ok) {
        setError(data.error || (authMode === 'login' ? 'Не удалось войти' : 'Не удалось зарегистрироваться'))
        return
      }
      setUser(data.user)
      setView('trainer')
      setAuthForm({ name: '', email: '', password: '', confirmPassword: '' })
    } catch (err) {
      setError(getNetworkErrorMessage(err, 'Ошибка сети'))
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
      setError(getNetworkErrorMessage(err, 'Ошибка сети при запуске сессии'))
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
      setError(getNetworkErrorMessage(err, 'Ошибка сети при отправке сообщения'))
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
      setError(getNetworkErrorMessage(err, 'Ошибка сети при получении разбора'))
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
      setError(getNetworkErrorMessage(err, 'Ошибка сети'))
    } finally {
      setLoading(false)
    }
  }

  async function refreshStatus() {
    setModelState('Проверяем модель...')
    await loadBootData()
    if (user?.id) {
      await loadProfile(user.id)
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
        <button className="model-pill model-button" onClick={refreshStatus} title="Обновить статус сервиса">
          {modelState}
        </button>
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
            profileStats={profileStats}
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
  profileStats,
  authMode,
  setAuthMode,
  authForm,
  setAuthForm,
  onSubmitAuth,
  onLogout,
  onOpenSession,
  loading,
}) {
  const passwordStrength = getPasswordStrength(authForm.password)
  const profileProgress = buildProfileProgress(user, sessions, profileStats)

  if (!user) {
    return (
      <section className="account-layout">
        <div className="account-auth-copy">
          <p className="eyebrow">Аккаунт</p>
          <h1 className="account-title">{authMode === 'login' ? 'Вход' : 'Регистрация'}</h1>
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
          {authMode === 'register' && (
            <>
              <label>
                Подтверждение пароля
                <input type="password" value={authForm.confirmPassword} onChange={(event) => setAuthForm({ ...authForm, confirmPassword: event.target.value })} />
              </label>
              <div className="password-meta">
                <span>Сложность: <strong>{passwordStrength.label}</strong></span>
                <span>{authForm.password.length} символов</span>
              </div>
            </>
          )}
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
        <h1 className="account-title">{user.name}</h1>
        <p className="muted">{user.email}</p>
        <div className="profile-progress">
          <div className="profile-progress-header">
            <strong>{profileProgress.level}</strong>
            <span>{profileProgress.percent}%</span>
          </div>
          <p className="muted">{profileProgress.stage}</p>
          <p>{profileProgress.description}</p>
          <ProgressSparkline scores={profileStats?.recent_scores || []} />
        </div>
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
