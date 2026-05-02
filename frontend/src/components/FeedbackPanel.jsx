function FeedbackPanel({ feedback, scenario, progress }) {
  const strengths = Array.isArray(feedback?.strengths) ? feedback.strengths : []
  const improvements = Array.isArray(feedback?.improvements) ? feedback.improvements : []
  const score = feedback?.score ?? 0
  const scorePercent = typeof feedback?.score_percent === 'number' ? feedback.score_percent : score * 10

  return (
    <aside className="feedback-panel">
      <div className="coach-header">
        <p className="eyebrow">Разбор</p>
        <h2>Навык</h2>
      </div>

      <div className="score-orbit">
        <strong>{feedback ? score : Math.round(progress / 10)}</strong>
        <span>{feedback ? `${scorePercent}%` : 'разогрев'}</span>
      </div>

      {!feedback ? (
        <div className="coach-empty">
          <p>{scenario?.description}</p>
          <ul>
            {(scenario?.goals || []).map((goal) => (
              <li key={goal}>{goal}</li>
            ))}
          </ul>
        </div>
      ) : (
        <>
          <FeedbackSection title="Сильные стороны" items={strengths} />
          <FeedbackSection title="Что усилить" items={improvements} />
          <div className="feedback-section">
            <strong>Резюме</strong>
            <p>{feedback.summary || 'Нет данных'}</p>
          </div>
          <div className="feedback-section highlight">
            <strong>Как можно было сказать</strong>
            <p>{feedback.better_example || 'Нет данных'}</p>
          </div>
        </>
      )}
    </aside>
  )
}

function FeedbackSection({ title, items }) {
  return (
    <div className="feedback-section">
      <strong>{title}</strong>
      {items.length ? (
        <ul>
          {items.map((item, idx) => (
            <li key={idx}>{item}</li>
          ))}
        </ul>
      ) : (
        <p>Нет данных</p>
      )}
    </div>
  )
}

export default FeedbackPanel
