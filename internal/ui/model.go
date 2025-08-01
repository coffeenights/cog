package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cog/internal/models"
	"cog/internal/storage"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sashabaranov/go-openai"
)

type FocusState int

const (
	FocusSidebar FocusState = iota
	FocusChat
)

// Model represents the main application state
type Model struct {
	viewport        viewport.Model
	textarea        textarea.Model
	conversations   []models.Conversation
	currentConvID   string
	convList        list.Model
	client          *openai.Client
	db              *storage.Database
	loading         bool
	err             error
	ready           bool
	focus           FocusState
	width           int
	height          int
	sidebarWidth    int
}

// ResponseMsg represents a message from the OpenAI API
type ResponseMsg struct {
	Content string
	Err     error
}

// NewModel creates a new UI model
func NewModel(client *openai.Client, db *storage.Database) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.Prompt = "┃ "
	ta.CharLimit = 2000
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(50, 20)
	vp.SetContent("Welcome to the AI Chat Interface!\nSelect a conversation or create a new one to start chatting.\n\n")

	// Load conversations from database
	conversations, err := db.LoadConversations()
	if err != nil {
		// Handle error - could be logged or shown to user
		conversations = []models.Conversation{}
	}

	// If no conversations exist, create a default one
	if len(conversations) == 0 {
		initialConv := NewConversation("New Chat")
		conversations = []models.Conversation{initialConv}
		if err := db.SaveConversation(initialConv); err != nil {
			// Handle error - could be logged
		}
	}

	// Set up conversation list
	items := make([]list.Item, len(conversations))
	for i, conv := range conversations {
		items[i] = conv
	}

	convList := list.New(items, list.NewDefaultDelegate(), 30, 20)
	convList.Title = "Conversations"
	convList.SetShowStatusBar(false)
	convList.SetFilteringEnabled(false)
	convList.SetShowHelp(false)

	// Set current conversation to the first one
	var currentConvID string
	if len(conversations) > 0 {
		currentConvID = conversations[0].ID
	}

	return &Model{
		textarea:      ta,
		viewport:      vp,
		conversations: conversations,
		currentConvID: currentConvID,
		convList:      convList,
		client:        client,
		db:            db,
		loading:       false,
		err:           nil,
		ready:         false,
		focus:         FocusChat,
		sidebarWidth:  30,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}

// GenerateConvID generates a unique conversation ID
func GenerateConvID() string {
	return fmt.Sprintf("conv_%d", time.Now().Unix())
}

// NewConversation creates a new conversation
func NewConversation(title string) models.Conversation {
	return models.Conversation{
		ID:       GenerateConvID(),
		Name:     title,
		Messages: []models.Message{},
		Created:  time.Now(),
	}
}

// Update handles UI events and state changes
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		clCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		chatWidth := msg.Width - m.sidebarWidth - 2
		chatHeight := msg.Height - 6

		if !m.ready {
			m.viewport = viewport.New(chatWidth, chatHeight)
			m.textarea.SetWidth(chatWidth - 2)
			m.convList.SetSize(m.sidebarWidth-2, chatHeight+3)
			m.ready = true
		} else {
			m.viewport.Width = chatWidth
			m.viewport.Height = chatHeight
			m.textarea.SetWidth(chatWidth - 2)
			m.convList.SetSize(m.sidebarWidth-2, chatHeight+3)
		}
		m.updateViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			if m.focus == FocusSidebar {
				m.focus = FocusChat
				m.textarea.Focus()
			} else {
				m.focus = FocusSidebar
				m.textarea.Blur()
			}
		case tea.KeyCtrlN:
			// Create new conversation
			newConv := NewConversation("New Chat")
			m.conversations = append(m.conversations, newConv)
			m.currentConvID = newConv.ID
			
			// Save to database
			if err := m.db.SaveConversation(newConv); err != nil {
				m.err = fmt.Errorf("failed to save conversation: %v", err)
			}
			
			m.updateConversationList()
			m.updateViewport()
			m.focus = FocusChat
			m.textarea.Focus()
		case tea.KeyEnter:
			if m.focus == FocusSidebar {
				// Switch to selected conversation
				if selectedItem, ok := m.convList.SelectedItem().(models.Conversation); ok {
					m.currentConvID = selectedItem.ID
					m.updateViewport()
					m.focus = FocusChat
					m.textarea.Focus()
				}
			} else if m.focus == FocusChat && !m.loading && strings.TrimSpace(m.textarea.Value()) != "" {
				// Send message
				userMsg := models.Message{
					Role:    "user",
					Content: strings.TrimSpace(m.textarea.Value()),
					Time:    time.Now(),
				}
				
				// Add message to current conversation
				for i := range m.conversations {
					if m.conversations[i].ID == m.currentConvID {
						m.conversations[i].Messages = append(m.conversations[i].Messages, userMsg)
						// Update conversation title if it's the first message
						if len(m.conversations[i].Messages) == 1 {
							title := userMsg.Content
							if utf8.RuneCountInString(title) > 30 {
								title = string([]rune(title)[:27]) + "..."
							}
							m.conversations[i].Name = title
						}
						
						// Save to database
						if err := m.db.SaveConversation(m.conversations[i]); err != nil {
							m.err = fmt.Errorf("failed to save conversation: %v", err)
						}
						break
					}
				}
				
				m.loading = true
				m.textarea.Reset()
				m.updateConversationList()
				m.updateViewport()
				return m, m.sendMessage(userMsg.Content)
			}
		}

	case ResponseMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			assistantMsg := models.Message{
				Role:    "assistant",
				Content: msg.Content,
				Time:    time.Now(),
			}
			// Add response to current conversation
			for i := range m.conversations {
				if m.conversations[i].ID == m.currentConvID {
					m.conversations[i].Messages = append(m.conversations[i].Messages, assistantMsg)
					
					// Save to database
					if err := m.db.SaveConversation(m.conversations[i]); err != nil {
						m.err = fmt.Errorf("failed to save conversation: %v", err)
					}
					break
				}
			}
		}
		m.updateConversationList()
		m.updateViewport()
	}

	// Update child components
	if m.focus == FocusChat {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	if m.focus == FocusSidebar {
		m.convList, clCmd = m.convList.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, clCmd)
}

func (m *Model) updateConversationList() {
	items := make([]list.Item, len(m.conversations))
	for i, conv := range m.conversations {
		items[i] = conv
	}
	m.convList.SetItems(items)
	
	// Select current conversation in list
	for i, conv := range m.conversations {
		if conv.ID == m.currentConvID {
			m.convList.Select(i)
			break
		}
	}
}

func (m *Model) getCurrentConversation() *models.Conversation {
	for i := range m.conversations {
		if m.conversations[i].ID == m.currentConvID {
			return &m.conversations[i]
		}
	}
	return nil
}

func (m *Model) updateViewport() {
	var content strings.Builder
	
	currentConv := m.getCurrentConversation()
	if currentConv == nil || len(currentConv.Messages) == 0 {
		content.WriteString("Welcome to the AI Chat Interface!\n")
		content.WriteString("Start typing to begin a conversation.\n\n")
		content.WriteString(HelpStyle.Render("Controls:\n"))
		content.WriteString(HelpStyle.Render("• Tab - Switch between sidebar and chat\n"))
		content.WriteString(HelpStyle.Render("• Ctrl+N - New conversation\n"))
		content.WriteString(HelpStyle.Render("• Enter - Send message / Select conversation\n"))
		content.WriteString(HelpStyle.Render("• Ctrl+C / Esc - Quit\n\n"))
	} else {
		for _, msg := range currentConv.Messages {
			timeStr := msg.Time.Format("15:04:05")
			
			if msg.Role == "user" {
				content.WriteString(MessageStyle.Render(
					UserStyle.Render("You") + " " + 
					lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("["+timeStr+"]") + "\n" +
					msg.Content + "\n\n",
				))
			} else {
				content.WriteString(MessageStyle.Render(
					AssistantStyle.Render("Assistant") + " " +
					lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("["+timeStr+"]") + "\n" +
					msg.Content + "\n\n",
				))
			}
		}
	}

	if m.loading {
		content.WriteString(MessageStyle.Render(
			LoadingStyle.Render("Assistant is typing...") + "\n",
		))
	}

	if m.err != nil {
		content.WriteString(MessageStyle.Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render("Error: " + m.err.Error() + "\n"),
		))
		m.err = nil
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

func (m Model) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		messages := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful AI assistant. Provide clear, concise, and helpful responses.",
			},
		}

		// Get current conversation messages
		currentConv := m.getCurrentConversation()
		if currentConv != nil {
			for _, msg := range currentConv.Messages {
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
		}

		resp, err := m.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			MaxTokens: 1000,
		})

		if err != nil {
			return ResponseMsg{Err: err}
		}

		if len(resp.Choices) == 0 {
			return ResponseMsg{Err: fmt.Errorf("no response from API")}
		}

		return ResponseMsg{Content: resp.Choices[0].Message.Content}
	}
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Create sidebar
	sidebarContent := m.convList.View()
	var sidebar string
	if m.focus == FocusSidebar {
		sidebar = SidebarFocusedStyle.Width(m.sidebarWidth).Height(m.height-1).Render(sidebarContent)
	} else {
		sidebar = SidebarStyle.Width(m.sidebarWidth).Height(m.height-1).Render(sidebarContent)
	}

	// Create chat area
	chatWidth := m.width - m.sidebarWidth - 2
	chatHeader := TitleStyle.Width(chatWidth).Render("AI Chat Interface")
	chatViewport := m.viewport.View()
	chatInput := m.textarea.View()
	
	chatArea := ChatStyle.Width(chatWidth).Render(
		fmt.Sprintf("%s\n%s\n%s", chatHeader, chatViewport, chatInput),
	)

	// Combine sidebar and chat area
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chatArea)
}