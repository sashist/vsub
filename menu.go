package main

import (
	"cmp"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	scrMenu screen = iota
	scrURL
	scrLoading
	scrServers
	scrCopyFmt
	scrSaveFmt
	scrSaveFile
)

var (
	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6")).
			Padding(0, 2)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

type loadedMsg struct {
	url   string
	links []string
	err   error
}

type uiModel struct {
	screen      screen
	hwid        string
	source      string
	links       []string
	statusMsg   string
	statusErr   bool
	cursor      int
	subCursor   int
	srvCursor   int
	srvOffset   int
	height      int
	saveFormat  string
	saveDefault string
	input       textinput.Model
	spin        spinner.Model
}

func newUIModel() uiModel {
	ti := textinput.New()
	ti.CharLimit = 2048
	ti.Width = 60

	sp := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("6"))),
	)

	return uiModel{
		screen: scrMenu,
		hwid:   deviceHWID(),
		input:  ti,
		spin:   sp,
	}
}

func runInteractive() error {
	_, err := tea.NewProgram(newUIModel(), tea.WithAltScreen()).Run()
	return err
}

func (m uiModel) Init() tea.Cmd {
	return nil
}

func (m *uiModel) setOK(msg string) {
	m.statusMsg = msg
	m.statusErr = false
}

func (m *uiModel) setErr(msg string) {
	m.statusMsg = msg
	m.statusErr = true
}

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m, nil

	case loadedMsg:
		if m.screen != scrLoading {
			return m, nil
		}
		m.screen = scrMenu
		if msg.err != nil {
			m.setErr(msg.err.Error())
			return m, nil
		}
		m.source = msg.url
		m.links = msg.links
		m.srvCursor = 0
		m.srvOffset = 0
		m.setOK(fmt.Sprintf("Loaded %d VLESS links", len(msg.links)))
		return m, nil
	}

	if m.screen == scrLoading {
		if k, ok := msg.(tea.KeyMsg); ok && k.String() == "ctrl+c" {
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		if m.screen == scrURL || m.screen == scrSaveFile {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case scrMenu:
		return m.updateMenu(key)
	case scrURL:
		return m.updateURLInput(key)
	case scrServers:
		return m.updateServers(key)
	case scrCopyFmt:
		return m.updateCopyFormat(key)
	case scrSaveFmt:
		return m.updateSaveFormat(key)
	case scrSaveFile:
		return m.updateSaveFile(key)
	}
	return m, nil
}

func (m uiModel) updateMenu(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	const count = 5
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.cursor = (m.cursor - 1 + count) % count
	case "down", "j":
		m.cursor = (m.cursor + 1) % count
	case "enter":
		return m.activateMenu(m.cursor)
	case "1", "2", "3", "4", "5":
		idx := int(key.String()[0] - '1')
		m.cursor = idx
		return m.activateMenu(idx)
	}
	return m, nil
}

func (m uiModel) activateMenu(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.screen = scrURL
		m.input.SetValue("")
		m.input.Placeholder = "https://..."
		m.input.Prompt = "> "
		return m, m.input.Focus()
	case 1:
		if len(m.links) == 0 {
			m.setErr("Load a subscription first (option 1)")
			return m, nil
		}
		m.screen = scrServers
	case 2:
		if len(m.links) == 0 {
			m.setErr("Load a subscription first (option 1)")
			return m, nil
		}
		m.subCursor = 0
		m.screen = scrCopyFmt
	case 3:
		if len(m.links) == 0 {
			m.setErr("Load a subscription first (option 1)")
			return m, nil
		}
		m.subCursor = 0
		m.screen = scrSaveFmt
	case 4:
		return m, tea.Quit
	}
	return m, nil
}

func (m uiModel) updateURLInput(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.screen = scrMenu
		m.input.Blur()
		return m, nil
	case "enter":
		rawURL := strings.TrimSpace(m.input.Value())
		m.input.Blur()
		if rawURL == "" {
			m.screen = scrMenu
			m.setErr("Cancelled (empty input)")
			return m, nil
		}
		m.screen = scrLoading
		return m, tea.Batch(m.spin.Tick, fetchCmd(rawURL))
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

func fetchCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		links, err := loadLinks(rawURL)
		return loadedMsg{url: rawURL, links: links, err: err}
	}
}

func (m uiModel) serverPageSize() int {
	if m.height <= 0 {
		return 12
	}
	n := (m.height - 8) / 2
	return min(max(n, 5), 18)
}

func (m *uiModel) clampServerScroll() {
	page := m.serverPageSize()
	if m.srvCursor < m.srvOffset {
		m.srvOffset = m.srvCursor
	}
	if m.srvCursor >= m.srvOffset+page {
		m.srvOffset = m.srvCursor - page + 1
	}
}

func (m uiModel) updateServers(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.links)
	page := m.serverPageSize()

	switch key.String() {
	case "esc", "q":
		m.screen = scrMenu
		return m, nil
	case "up", "k":
		if m.srvCursor > 0 {
			m.srvCursor--
		}
	case "down", "j":
		if m.srvCursor < total-1 {
			m.srvCursor++
		}
	case "pgup", "ctrl+u":
		m.srvCursor = max(0, m.srvCursor-page)
	case "pgdown", "ctrl+d":
		m.srvCursor = min(total-1, m.srvCursor+page)
	case "home", "g":
		m.srvCursor = 0
	case "end", "G":
		m.srvCursor = total - 1
	case "enter":
		link := m.links[m.srvCursor]
		m.screen = scrMenu
		if err := copyToClipboard(link + "\n"); err != nil {
			m.setErr(err.Error())
			return m, nil
		}
		m.setOK("Copied: " + linkName(link))
		return m, nil
	}
	m.clampServerScroll()
	return m, nil
}

func (m uiModel) updateCopyFormat(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	picked, done := navigateSub(&m.subCursor, 2, key)
	if !done {
		return m, nil
	}
	m.screen = scrMenu
	if picked < 0 {
		return m, nil
	}

	fmtName := "txt"
	if picked == 1 {
		fmtName = "base64"
	}
	text, err := formatLinks(m.links, fmtName)
	if err != nil {
		m.setErr("format error: " + err.Error())
		return m, nil
	}
	if err := copyToClipboard(text); err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	m.setOK(fmt.Sprintf("Copied %d links (%s) to clipboard", len(m.links), fmtName))
	return m, nil
}

func (m uiModel) updateSaveFormat(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	picked, done := navigateSub(&m.subCursor, 3, key)
	if !done {
		return m, nil
	}
	if picked < 0 {
		m.screen = scrMenu
		return m, nil
	}

	formats := []struct {
		name string
		file string
	}{
		{"txt", "vless_servers.txt"},
		{"base64", "vless_sub.b64"},
		{"json", "vless_servers.json"},
	}
	chosen := formats[picked]
	m.saveFormat = chosen.name
	m.saveDefault = chosen.file
	m.screen = scrSaveFile
	m.input.SetValue(chosen.file)
	m.input.Placeholder = chosen.file
	m.input.Prompt = "> "
	m.input.CursorEnd()
	return m, m.input.Focus()
}

func (m uiModel) updateSaveFile(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.screen = scrMenu
		m.input.Blur()
		return m, nil
	case "enter":
		target := expandHome(cmp.Or(strings.TrimSpace(m.input.Value()), m.saveDefault))
		m.input.Blur()
		m.screen = scrMenu

		text, err := formatLinks(m.links, m.saveFormat)
		if err != nil {
			m.setErr("format error: " + err.Error())
			return m, nil
		}
		if err := os.WriteFile(target, []byte(text), 0644); err != nil {
			m.setErr("save error: " + err.Error())
			return m, nil
		}
		m.setOK(fmt.Sprintf("Saved %d links to %s (%s)", len(m.links), target, m.saveFormat))
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

func navigateSub(cursor *int, count int, key tea.KeyMsg) (picked int, done bool) {
	switch key.String() {
	case "esc", "q":
		return -1, true
	case "up", "k":
		*cursor = (*cursor - 1 + count) % count
	case "down", "j":
		*cursor = (*cursor + 1) % count
	case "enter":
		return *cursor, true
	default:
		s := key.String()
		if len(s) == 1 && s[0] >= '1' && s[0] <= byte('0'+count) {
			idx := int(s[0] - '1')
			*cursor = idx
			return idx, true
		}
	}
	return 0, false
}

func (m uiModel) View() string {
	switch m.screen {
	case scrURL:
		return m.viewPrompt("Paste subscription URL", "Enter URL and press Enter (Esc to cancel):")
	case scrLoading:
		return "\n  " + m.spin.View() + " Fetching subscription...\n"
	case scrServers:
		return m.viewServers()
	case scrCopyFmt:
		return viewSubMenu("Clipboard format", []string{
			"TXT — plain vless:// list",
			"Base64 — encoded subscription",
		}, m.subCursor)
	case scrSaveFmt:
		return viewSubMenu("File format", []string{
			"TXT — plain vless:// list (vless_servers.txt)",
			"Base64 — encoded subscription (vless_sub.b64)",
			"JSON — server list with names (vless_servers.json)",
		}, m.subCursor)
	case scrSaveFile:
		return m.viewPrompt("Save to file ("+m.saveFormat+")", "File path (Enter to save, Esc to cancel):")
	default:
		return m.viewMain()
	}
}

func (m uiModel) viewMain() string {
	var b strings.Builder

	header := headerBoxStyle.Render(titleStyle.Render("vsub") + "  " + dimStyle.Render("VLESS subscription parser"))
	b.WriteString("\n")
	b.WriteString(indentBlock(header, "  "))
	b.WriteString("\n\n")

	src := dimStyle.Render("none")
	if m.source != "" {
		src = truncateRunes(m.source, 60)
	}
	cnt := dimStyle.Render("0")
	if len(m.links) > 0 {
		cnt = okStyle.Bold(true).Render(fmt.Sprintf("%d", len(m.links)))
	}

	b.WriteString(fmt.Sprintf("  %s  %s\n", dimStyle.Render("HWID   "), m.hwid))
	b.WriteString(fmt.Sprintf("  %s  %s\n", dimStyle.Render("Source "), src))
	b.WriteString(fmt.Sprintf("  %s  %s\n\n", dimStyle.Render("Servers"), cnt))

	renderOptions(&b, []string{
		"Paste subscription URL",
		"Server list (select & copy)",
		"Copy all to clipboard",
		"Save to file (txt / base64 / json)",
		"Quit",
	}, m.cursor)

	b.WriteString("\n  " + dimStyle.Render("↑/↓ + Enter · 1-5 · q quit") + "\n")
	if m.statusMsg != "" {
		if m.statusErr {
			b.WriteString("\n  " + errStyle.Render("✗ "+m.statusMsg) + "\n")
		} else {
			b.WriteString("\n  " + okStyle.Render("✓ "+m.statusMsg) + "\n")
		}
	}
	return b.String()
}

func (m uiModel) viewPrompt(title, hint string) string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render(title) + "\n")
	b.WriteString("  " + dimStyle.Render(hint) + "\n\n")
	b.WriteString("  " + m.input.View() + "\n")
	return b.String()
}

func (m uiModel) viewServers() string {
	var b strings.Builder
	page := m.serverPageSize()
	end := min(m.srvOffset+page, len(m.links))

	header := fmt.Sprintf("Servers (%d–%d of %d)", m.srvOffset+1, end, len(m.links))
	b.WriteString("\n  " + titleStyle.Render(header) + "\n")
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", 44)) + "\n\n")

	for i := m.srvOffset; i < end; i++ {
		link := m.links[i]
		name := linkName(link)
		info := linkSummary(link)
		if i == m.srvCursor {
			b.WriteString("  " + activeStyle.Render(fmt.Sprintf("❯ [%d] %s", i+1, name)) + "\n")
			b.WriteString("      " + dimStyle.Render(info) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("    %s %s\n", dimStyle.Render(fmt.Sprintf("[%d]", i+1)), name))
			b.WriteString("      " + dimStyle.Render(info) + "\n")
		}
	}

	b.WriteString("\n  " + dimStyle.Render("↑/↓ · PgUp/PgDn · Enter copy · Esc/q back") + "\n")
	return b.String()
}

func viewSubMenu(title string, options []string, cursor int) string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render(title) + "\n")
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", 44)) + "\n\n")
	renderOptions(&b, options, cursor)
	b.WriteString(fmt.Sprintf("\n  %s\n", dimStyle.Render(fmt.Sprintf("↑/↓ + Enter · 1-%d · Esc/q back", len(options)))))
	return b.String()
}

func renderOptions(b *strings.Builder, options []string, cursor int) {
	for i, label := range options {
		if i == cursor {
			b.WriteString("  " + activeStyle.Render(fmt.Sprintf("❯ [%d] %s", i+1, label)) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("    %s %s\n", dimStyle.Render(fmt.Sprintf("[%d]", i+1)), label))
		}
	}
}

func indentBlock(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func linkName(link string) string {
	idx := strings.LastIndex(link, "#")
	if idx == -1 || idx == len(link)-1 {
		return "Server"
	}
	name, err := url.PathUnescape(link[idx+1:])
	if err != nil || name == "" {
		return link[idx+1:]
	}
	return name
}

func linkSummary(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return truncateRunes(raw, 60)
	}
	q := u.Query()
	net := cmp.Or(q.Get("type"), "tcp")
	sec := cmp.Or(q.Get("security"), "none")
	return fmt.Sprintf("%s · %s/%s", u.Host, net, sec)
}

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
