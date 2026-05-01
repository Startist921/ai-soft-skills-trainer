import { useState } from 'react'

function MessageInput({ onSend, disabled }) {
  const [text, setText] = useState('')

  const handleSend = () => {
    if (!text.trim()) return
    onSend(text.trim())
    setText('')
  }

  const onKeyDown = (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="message-input-row">
      <textarea
        value={text}
        disabled={disabled}
        onChange={(event) => setText(event.target.value)}
        onKeyDown={onKeyDown}
        placeholder="Напишите ответ как в реальном разговоре..."
        rows={3}
      />
      <button onClick={handleSend} disabled={disabled || !text.trim()} title="Отправить ответ">
        Отправить
      </button>
    </div>
  )
}

export default MessageInput
