function ScenarioSelector({ scenarios, onSelect, onRandom, loading, user, remaining }) {
  return (
    <section className="scenario-zone">
      <div className="scenario-intro">
        <p className="eyebrow">Меню диалогов</p>
        <h1>Выберите сцену или запустите случайную.</h1>
        <p>
          {user
            ? `Сегодня осталось тренировок: ${remaining}. Каждый запуск получает новый контекст.`
            : 'Войдите в кабинет, чтобы запускать тренировки и сохранять историю.'}
        </p>
        <button className="finish-button" onClick={onRandom} disabled={loading}>
          Случайный диалог
        </button>
      </div>

      <div className="scenario-grid">
        {scenarios.map((scenario) => (
          <button
            key={scenario.id}
            className="scenario-card"
            onClick={() => onSelect(scenario)}
            disabled={loading}
          >
            <span className="scenario-meta">{scenario.skill} · {scenario.difficulty}</span>
            <strong>{scenario.title}</strong>
            <span>{scenario.description}</span>
            <span className="goal-strip">
              {(scenario.goals || []).slice(0, 3).map((goal) => (
                <em key={goal}>{goal}</em>
              ))}
            </span>
          </button>
        ))}
      </div>
    </section>
  )
}

export default ScenarioSelector
