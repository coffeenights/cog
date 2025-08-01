package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/list"
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

type Conversation struct {
	ID       string
	Name     string
	Messages []Message
	Created  time.Time
}

func (c Conversation) FilterValue() string { return c.Name }
func (c Conversation) Title() string       { return c.Name }
func (c Conversation) Description() string {
	if len(c.Messages) == 0 {
		return "New conversation"
	}
	lastMsg := c.Messages[len(c.Messages)-1]
	preview := lastMsg.Content
	if utf8.RuneCountInString(preview) > 50 {
		preview = string([]rune(preview)[:47]) + "..."
	}
	return preview
}

type focusState int

const (
	focusSidebar focusState = iota
	focusChat
)

type model struct {
	viewport        viewport.Model
	textarea        textarea.Model
	conversations   []Conversation
	currentConvID   string
	convList        list.Model
	client          *openai.Client
	db              *Database
	loading         bool
	err             error
	ready           bool
	focus           focusState
	width           int
	height          int
	sidebarWidth    int
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

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#444444"))

	sidebarFocusedStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#25A065"))

	chatStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
)

func generateConvID() string {
	return fmt.Sprintf("conv_%d", time.Now().Unix())
}

func newConversation(title string) Conversation {
	return Conversation{
		ID:       generateConvID(),
		Name:     title,
		Messages: []Message{},
		Created:  time.Now(),
	}
}

func initialModel() model {
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

	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	// Initialize database
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Failed to get user home directory:", err)
	}
	dbPath := filepath.Join(homeDir, ".cog", "conversations.db")
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal("Failed to create database directory:", err)
	}

	db, err := NewDatabase(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Load conversations from database
	conversations, err := db.LoadConversations()
	if err != nil {
		log.Fatal("Failed to load conversations:", err)
	}

	// If no conversations exist, create a default one
	if len(conversations) == 0 {
		initialConv := newConversation("New Chat")
		conversations = []Conversation{initialConv}
		if err := db.SaveConversation(initialConv); err != nil {
			log.Printf("Failed to save initial conversation: %v", err)
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

	m := model{
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
		focus:         focusChat,
		sidebarWidth:  30,
	}

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.focus == focusSidebar {
				m.focus = focusChat
				m.textarea.Focus()
			} else {
				m.focus = focusSidebar
				m.textarea.Blur()
			}
		case tea.KeyCtrlN:
			// Create new conversation
			newConv := newConversation("New Chat")
			m.conversations = append(m.conversations, newConv)
			m.currentConvID = newConv.ID
			
			// Save to database
			if err := m.db.SaveConversation(newConv); err != nil {
				m.err = fmt.Errorf("failed to save conversation: %v", err)
			}
			
			m.updateConversationList()
			m.updateViewport()
			m.focus = focusChat
			m.textarea.Focus()
		case tea.KeyEnter:
			if m.focus == focusSidebar {
				// Switch to selected conversation
				if selectedItem, ok := m.convList.SelectedItem().(Conversation); ok {
					m.currentConvID = selectedItem.ID
					m.updateViewport()
					m.focus = focusChat
					m.textarea.Focus()
				}
			} else if m.focus == focusChat && !m.loading && strings.TrimSpace(m.textarea.Value()) != "" {
				// Send message
				userMsg := Message{
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
	if m.focus == focusChat {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	if m.focus == focusSidebar {
		m.convList, clCmd = m.convList.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, clCmd)
}

func (m *model) updateConversationList() {
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

func (m *model) getCurrentConversation() *Conversation {
	for i := range m.conversations {
		if m.conversations[i].ID == m.currentConvID {
			return &m.conversations[i]
		}
	}
	return nil
}

func (m *model) updateViewport() {
	var content strings.Builder
	
	currentConv := m.getCurrentConversation()
	if currentConv == nil || len(currentConv.Messages) == 0 {
		content.WriteString("Welcome to the AI Chat Interface!\n")
		content.WriteString("Start typing to begin a conversation.\n\n")
		content.WriteString(helpStyle.Render("Controls:\n"))
		content.WriteString(helpStyle.Render("• Tab - Switch between sidebar and chat\n"))
		content.WriteString(helpStyle.Render("• Ctrl+N - New conversation\n"))
		content.WriteString(helpStyle.Render("• Enter - Send message / Select conversation\n"))
		content.WriteString(helpStyle.Render("• Ctrl+C / Esc - Quit\n\n"))
	} else {
		for _, msg := range currentConv.Messages {
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

	// Create sidebar
	sidebarContent := m.convList.View()
	var sidebar string
	if m.focus == focusSidebar {
		sidebar = sidebarFocusedStyle.Width(m.sidebarWidth).Height(m.height-1).Render(sidebarContent)
	} else {
		sidebar = sidebarStyle.Width(m.sidebarWidth).Height(m.height-1).Render(sidebarContent)
	}

	// Create chat area
	chatWidth := m.width - m.sidebarWidth - 2
	chatHeader := titleStyle.Width(chatWidth).Render("AI Chat Interface")
	chatViewport := m.viewport.View()
	chatInput := m.textarea.View()
	
	chatArea := chatStyle.Width(chatWidth).Render(
		fmt.Sprintf("%s\n%s\n%s", chatHeader, chatViewport, chatInput),
	)

	// Combine sidebar and chat area
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chatArea)
}

func main() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	model := initialModel()
	defer model.db.Close()

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}