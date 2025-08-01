# Cog - AI Chat Interface

A terminal-based chat interface built with Bubble Tea for interacting with OpenAI's GPT models.

## Features

- 📱 Beautiful terminal UI with Bubble Tea
- 💬 Real-time chat with OpenAI GPT
- 🗂️ Multiple conversation management with SQLite storage
- ⌨️ Intuitive keyboard controls
- 🎨 Syntax highlighting and styled messages
- ⏱️ Message timestamps
- 🔄 Loading indicators
- ❌ Error handling
- 💾 Persistent conversation history

## Project Structure

```
cog/
├── cmd/cog/           # Application entry point
│   └── main.go
├── internal/          # Private application code
│   ├── models/        # Data models
│   │   └── conversation.go
│   ├── storage/       # Database operations
│   │   └── database.go
│   └── ui/           # User interface components
│       ├── model.go
│       └── styles.go
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

## Setup

1. Install dependencies:
```bash
go mod tidy
```

2. Set your OpenAI API key:
```bash
export OPENAI_API_KEY="your-api-key-here"
```

3. Build the application:
```bash
go build -o bin/cog ./cmd/cog
```

4. Run the application:
```bash
./bin/cog
```

Or run directly:
```bash
go run ./cmd/cog
```

## Usage

- **Tab** - Switch between sidebar and chat area
- **Arrow keys** - Navigate conversations (when sidebar focused)
- **Enter** - Send message (in chat) or select conversation (in sidebar)
- **Ctrl+N** - Create new conversation
- **Ctrl+C** / **Esc** - Quit application

## Features

### Conversation Management
- Multiple persistent conversations stored in SQLite
- Automatic conversation titling from first message
- New conversations appear at the bottom of the list
- Easy switching between conversations

### Database Storage
- SQLite database stored in `~/.cog/conversations.db`
- Persistent message history across sessions
- Automatic database schema creation and migration

### UI Features
- Split-pane interface with conversation sidebar
- Real-time message display with timestamps
- Loading indicators during API calls
- Error handling and display

## Requirements

- Go 1.23+
- OpenAI API key
- Terminal with color support