function MessageList({ messages, aiTyping }) {
  return (
    <div className="message-list">
      {messages.map((message, index) => (
        <article key={message.id || index} className={`message ${message.role}`}>
          <span>{message.role === 'user' ? 'Вы' : 'Собеседник'}</span>
          <p>{message.text}</p>
        </article>
      ))}
      {aiTyping && (
        <article className="message assistant typing">
          <span>Собеседник</span>
          <p>Думает над ответом...</p>
        </article>
      )}
    </div>
  )
}

export default MessageList
