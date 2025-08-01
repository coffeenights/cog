# Cog - AI Chat Interface

A terminal-based chat interface built with Bubble Tea for interacting with OpenAI's GPT models.

## Features

- 📱 Beautiful terminal UI with Bubble Tea
- 💬 Real-time chat with OpenAI GPT
- ⌨️ Intuitive keyboard controls
- 🎨 Syntax highlighting and styled messages
- ⏱️ Message timestamps
- 🔄 Loading indicators
- ❌ Error handling

## Setup

1. Install dependencies:
```bash
go mod tidy
```

2. Set your OpenAI API key:
```bash
export OPENAI_API_KEY="your-api-key-here"
```

3. Run the application:
```bash
go run main.go
```

## Usage

- Type your message and press **Enter** to send
- Press **Ctrl+C** or **Esc** to quit
- The interface shows conversation history with timestamps
- Loading indicator appears while waiting for responses

## Controls

- **Enter**: Send message
- **Ctrl+C** / **Esc**: Quit application
- **Up/Down arrows**: Scroll through chat history (when focused on viewport)

## Requirements

- Go 1.21+
- OpenAI API key
- Terminal with color support