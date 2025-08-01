package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sashabaranov/go-openai"
)

type Message struct {
	Role    string
	Content string
	Time    time.Time
}

type model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	messages    []Message
	client      *openai.Client
	loading     bool
	err         error
	ready       bool
}

type responseMsg struct {
	content string
	err     error
}

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB347")).
			Bold(true)

	messageStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			MarginBottom(1)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB347")).
			Italic(true)
)

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 2000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to the AI Chat Interface!\nType your message below and press Enter to send.\n\n")

	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	return model{
		textarea:    ta,
		viewport:    vp,
		messages:    []Message{},
		client:      client,
		loading:     false,
		err:         nil,
		ready:       false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.textarea.SetWidth(msg.Width - 4)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
			m.textarea.SetWidth(msg.Width - 4)
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if !m.loading && strings.TrimSpace(m.textarea.Value()) != "" {
				userMsg := Message{
					Role:    "user",
					Content: strings.TrimSpace(m.textarea.Value()),
					Time:    time.Now(),
				}
				m.messages = append(m.messages, userMsg)
				m.loading = true
				m.textarea.Reset()
				m.updateViewport()
				return m, m.sendMessage(userMsg.Content)
			}
		}

	case responseMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			assistantMsg := Message{
				Role:    "assistant",
				Content: msg.content,
				Time:    time.Now(),
			}
			m.messages = append(m.messages, assistantMsg)
		}
		m.updateViewport()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *model) updateViewport() {
	var content strings.Builder
	
	content.WriteString("Welcome to the AI Chat Interface!\n")
	content.WriteString("Type your message below and press Enter to send.\n")
	content.WriteString("Press Ctrl+C or Esc to quit.\n\n")

	for _, msg := range m.messages {
		timeStr := msg.Time.Format("15:04:05")
		
		if msg.Role == "user" {
			content.WriteString(messageStyle.Render(
				userStyle.Render("You") + " " + 
				lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("["+timeStr+"]") + "\n" +
				msg.Content + "\n\n",
			))
		} else {
			content.WriteString(messageStyle.Render(
				assistantStyle.Render("Assistant") + " " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("["+timeStr+"]") + "\n" +
				msg.Content + "\n\n",
			))
		}
	}

	if m.loading {
		content.WriteString(messageStyle.Render(
			loadingStyle.Render("Assistant is typing...") + "\n",
		))
	}

	if m.err != nil {
		content.WriteString(messageStyle.Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render("Error: " + m.err.Error() + "\n"),
		))
		m.err = nil
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

func (m model) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		messages := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful AI assistant. Provide clear, concise, and helpful responses.",
			},
		}

		for _, msg := range m.messages {
			var role string
			if msg.Role == "user" {
				role = openai.ChatMessageRoleUser
			} else {
				role = openai.ChatMessageRoleAssistant
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    role,
				Content: msg.Content,
			})
		}

		resp, err := m.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			MaxTokens: 1000,
		})

		if err != nil {
			return responseMsg{err: err}
		}

		if len(resp.Choices) == 0 {
			return responseMsg{err: fmt.Errorf("no response from API")}
		}

		return responseMsg{content: resp.Choices[0].Message.Content}
	}
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	return fmt.Sprintf(
		"%s\n%s\n%s",
		titleStyle.Render("AI Chat Interface"),
		m.viewport.View(),
		m.textarea.View(),
	)
}

func main() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}