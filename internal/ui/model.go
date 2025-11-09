package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FlameInTheDark/khm/internal/knownhosts"
)

type Model struct {
	list       list.Model
	input      textinput.Model
	collection *knownhosts.HostCollection
	version    string

	filterText string

	showFilter    bool
	showStash     bool
	showConfirm   bool
	showHelp      bool
	showDetails   bool
	showStashView bool

	moveTarget textinput.Model

	selectedHost       string
	selectedIndex      int
	baseKnownHostsPath string

	status string
	width  int
	height int
}

type hostItem struct {
	addressLabel string

	hosts []*knownhosts.Host
}

func (i hostItem) Title() string {

	if len(i.hosts) == 0 {

		return i.addressLabel

	}

	host := i.hosts[0]

	// Hashed hosts: show a hash prefix
	if host.IsHashed && host.HashValue != "" {
		title := host.HashValue
		if len(host.HashValue) > 20 {
			title = host.HashValue[:20] + "..."
		}
		types := collectTypes(i.hosts)
		if types != "" {
			title += "  [" + types + "]"
		}

		// Show total number of keys for this hashed host when more than one
		if len(i.hosts) > 1 {
			title += fmt.Sprintf(" (%d keys)", len(i.hosts))
		}

		return title
	}

	// Regular hosts: show first address
	addr := i.addressLabel
	if addr == "" && len(host.Addresses) > 0 {
		addr = host.Addresses[0]
	}

	title := addr

	// Append unique key types inline on the right
	types := collectTypes(i.hosts)
	if types != "" {
		title += "  [" + types + "]"
	}

	// Show total number of keys for this host when more than one
	if len(i.hosts) > 1 {
		title += fmt.Sprintf(" (%d keys)", len(i.hosts))
	}

	return title
}

func (i hostItem) Description() string {

	if len(i.hosts) == 0 {

		return "No hosts found"

	}

	host := i.hosts[0]

	// Keep description minimal so that the list title carries the important info.
	// Only show comment here if present.
	if host.Comment != "" {
		return host.Comment
	}

	return ""

}

func (i hostItem) FilterValue() string {

	hostTypes := collectTypes(i.hosts)
	if hostTypes != "" {
		return i.addressLabel + " " + hostTypes
	}
	return i.addressLabel
}

func collectTypes(hosts []*knownhosts.Host) string {
	if len(hosts) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	order := make([]string, 0)
	for _, h := range hosts {
		if h == nil || h.Type == "" {
			continue
		}
		if _, ok := seen[h.Type]; !ok {
			seen[h.Type] = struct{}{}
			order = append(order, h.Type)
		}
	}
	return strings.Join(order, ",")
}

func NewModel(collection *knownhosts.HostCollection, version string) *Model {

	items := make([]list.Item, 0)

	// Build a stable list of unique address labels from the collection.
	// HostCollection.Hosts maps address -> []*Host; we sort the keys for stability.
	addresses := make([]string, 0, len(collection.Hosts))

	for addr := range collection.Hosts {

		addresses = append(addresses, addr)

	}

	sort.Strings(addresses)

	for _, addr := range addresses {

		hosts := collection.Hosts[addr]

		if len(hosts) == 0 {
			continue
		}

		// We treat each address key as a label pointing at its group of hosts.
		items = append(items, hostItem{

			addressLabel: addr,

			hosts: hosts,
		})

	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#A78BFA")).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#C4B5FD"))
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	// Reduce vertical gaps between items
	delegate.SetSpacing(0)

	listModel := list.New(items, delegate, 80, 20)
	listModel.Title = "SSH Known Hosts Manager"
	if version != "" {
		listModel.Title += " " + version
	}
	listModel.SetShowHelp(false)
	// Disable built-in status bar and pagination dots; we render our own status bar.
	listModel.SetShowStatusBar(false)
	listModel.SetShowPagination(false)
	listModel.SetShowFilter(false)

	// Style status bar for better contrast
	listModel.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Background(lipgloss.Color("#111827")).
		Padding(0, 1)

	listModel.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Padding(0, 1)

	input := textinput.New()
	input.Placeholder = "Type to filter • Enter to close • Esc to clear"
	input.CharLimit = 100
	input.Width = 40

	moveTarget := textinput.New()
	moveTarget.Placeholder = "Stash file path... (Enter to confirm, Esc to cancel, empty = default stash_hosts)"
	moveTarget.CharLimit = 200
	moveTarget.Width = 50

	return &Model{
		list:               listModel,
		input:              input,
		collection:         collection,
		version:            version,
		moveTarget:         moveTarget,
		baseKnownHostsPath: collection.File,
		status:             "Ready",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) rebuildList() {
	// capture previous selection
	prevIndex := m.list.Index()
	prevLabel := ""
	if sel := m.list.SelectedItem(); sel != nil {
		if hi, ok := sel.(hostItem); ok {
			prevLabel = hi.addressLabel
		}
	}

	items := make([]list.Item, 0)
	filter := strings.ToLower(strings.TrimSpace(m.filterText))
	terms := strings.Fields(filter)

	matches := func(label string, hosts []*knownhosts.Host) bool {
		if len(terms) == 0 {
			return true
		}

		// searchable fields: label, all addresses, type, comment, hash
		fields := []string{strings.ToLower(label)}
		for _, h := range hosts {
			for _, a := range h.Addresses {
				fields = append(fields, strings.ToLower(a))
			}
			if h.Type != "" {
				fields = append(fields, strings.ToLower(h.Type))
			}
			if h.Comment != "" {
				fields = append(fields, strings.ToLower(h.Comment))
			}
			if h.IsHashed && h.HashValue != "" {
				fields = append(fields, strings.ToLower(h.HashValue))
			}
		}

		// all terms must match at least one field (AND of ORs)
		for _, t := range terms {
			found := false
			for _, f := range fields {
				if strings.Contains(f, t) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}

	// stable ordering of labels
	addresses := make([]string, 0, len(m.collection.Hosts))
	for addr := range m.collection.Hosts {
		addresses = append(addresses, addr)
	}
	sort.Strings(addresses)

	for _, addr := range addresses {
		hosts := m.collection.Hosts[addr]
		if matches(addr, hosts) {
			items = append(items, hostItem{
				addressLabel: addr,
				hosts:        hosts,
			})
		}
	}

	m.list.SetItems(items)

	// restore selection
	if len(items) == 0 {
		return
	}

	// try to select previous label if still present
	if prevLabel != "" {
		for i, it := range items {
			if hi, ok := it.(hostItem); ok && hi.addressLabel == prevLabel {
				m.list.Select(i)
				return
			}
		}
	}

	// otherwise clamp to nearest valid index
	if prevIndex < 0 {
		prevIndex = 0
	}
	if prevIndex >= len(items) {
		prevIndex = len(items) - 1
	}
	m.list.Select(prevIndex)
}

// updateListSize recalculates the list's height to use available space,
// leaving room for the status bar and any visible input rows.
func (m *Model) updateListSize() {
	if m.width == 0 || m.height == 0 {
		return
	}
	reserved := 1 // status bar
	if m.showFilter {
		reserved += 1
	}
	if m.showStash {
		reserved += 1
	}
	available := m.height - reserved
	if available < 5 {
		available = 5
	}
	m.list.SetSize(m.width, available)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateListSize()
		return m, nil

	case tea.KeyMsg:
		// If confirmation dialog is open, only handle Enter/Esc
		if m.showConfirm {
			switch msg.String() {
			case "enter":
				m.showConfirm = false
				m.updateListSize()
				return m, m.deleteSelectedHost()
			case "esc":
				m.showConfirm = false
				m.status = "Delete canceled"
				m.updateListSize()
				return m, nil
			}
			return m, nil
		}

		// If details view is open, Enter/Esc toggle/close it without affecting list navigation
		if m.showDetails {
			switch msg.String() {
			case "enter", "esc":
				m.showDetails = false
				m.status = "Closed host details"
				m.updateListSize()
				return m, nil
			}
			// Ignore other keys while in details mode
			return m, nil
		}

		// Handle global keys
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "/":
			if !m.showFilter && !m.showStash && !m.showStashView {
				m.showFilter = true
				m.input.Focus()
				m.updateListSize()
				return m, textinput.Blink
			}

		case "enter":
			if m.showFilter {
				m.showFilter = false
				m.filterText = m.input.Value()
				m.rebuildList()
				m.updateListSize()
				return m, nil
			} else if m.showStash {
				m.showStash = false
				m.updateListSize()
				return m, m.stashSelectedHost()
			} else {
				// Toggle host details for the currently selected item
				if m.list.SelectedItem() == nil || len(m.list.Items()) == 0 {
					m.status = "No host selected"
					return m, nil
				}
				m.showDetails = !m.showDetails
				if m.showDetails {
					m.status = "Showing host details (Enter/Esc to close)"
				} else {
					m.status = "Closed host details"
				}
				m.updateListSize()
				return m, nil
			}

		case "d":
			if !m.showFilter && !m.showStash && !m.showStashView {
				if m.list.SelectedItem() == nil || len(m.list.Items()) == 0 {
					m.status = "No host selected"
					return m, nil
				}
				m.showConfirm = true
				m.updateListSize()
				return m, nil
			}

		case "s":
			if !m.showFilter && !m.showStash && !m.showStashView {
				m.showStash = true
				m.moveTarget.Focus()
				m.updateListSize()
				return m, textinput.Blink
			}

		case "t":
			// Toggle stash view: show hosts from stash_hosts instead of known_hosts
			if !m.showFilter && !m.showStash {
				m.showStashView = !m.showStashView
				if m.showStashView {
					if err := m.loadStash(); err != nil {
						m.showStashView = false
						m.status = fmt.Sprintf("Error loading stash: %v", err)
					} else {
						m.status = "Showing stash_hosts (t to switch back)"
						m.list.Title = "SSH Known Hosts Manager (stash_hosts)"
					}
				} else {
					m.status = "Switched to known_hosts view"
					m.reloadKnownHosts()
					m.list.Title = "SSH Known Hosts Manager (known_hosts)"
				}
				m.updateListSize()
				return m, nil
			}

		case "r":
			// Restore selected host from stash back to known_hosts when in stash view
			if m.showStashView && !m.showFilter && !m.showStash {
				return m, m.restoreSelectedFromStash()
			}

		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		}

		// Handle input focus
		if m.showFilter {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			// live filtering as you type
			m.filterText = m.input.Value()
			m.rebuildList()
			return m, cmd
		} else if m.showStash {
			var cmd tea.Cmd
			m.moveTarget, cmd = m.moveTarget.Update(msg)
			return m, cmd
		}

		// Global ESC handling (after other keys)
		if msg.String() == "esc" {
			if m.showHelp {
				m.showHelp = false
				m.status = "Closed help"
				return m, nil
			}
			if m.showFilter {
				m.showFilter = false
				m.input.SetValue("")
				m.filterText = ""
				m.rebuildList()
				m.status = "Filter cleared"
				m.updateListSize()
				return m, nil
			}
			if m.showStash {
				m.showStash = false
				m.moveTarget.SetValue("")
				m.updateListSize()
				m.status = "Stash canceled"
				return m, nil
			}
			if m.showStashView {
				// Exit stash view back to known_hosts view
				m.showStashView = false
				m.status = "Switched to known_hosts view"
				m.reloadKnownHosts()
				m.updateListSize()
				return m, nil
			}
			if strings.TrimSpace(m.filterText) != "" {
				// allow clearing active filter even when prompt is closed
				m.filterText = ""
				m.rebuildList()
				m.status = "Filter cleared"
				return m, nil
			}
			return m, nil
		}
	}

	// Update list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// padToBottom appends blank lines so that the next line (status bar)
// is rendered at the very bottom row of the terminal window.
func (m Model) padToBottom(content string) string {
	if m.height <= 0 {
		return content
	}
	lines := 1
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines++
		}
	}
	usable := m.height - 1 // reserve one line for status bar
	if usable < 0 {
		usable = 0
	}
	if lines < usable {
		padding := usable - lines
		return content + strings.Repeat("\n", padding)
	}
	return content
}

func (m Model) View() string {
	var view strings.Builder

	// Main content
	if m.showHelp {
		view.WriteString(m.renderHelp())
	} else if m.showConfirm {
		view.WriteString(m.renderConfirm())
	} else if m.showDetails {
		view.WriteString(m.renderDetails())
	} else if len(m.list.Items()) == 0 {
		view.WriteString(m.renderEmptyState())
	} else {
		view.WriteString(m.list.View())
	}

	// Filter input
	if m.showFilter {
		view.WriteString("\n")
		view.WriteString(m.renderFilter())
	}

	// Stash input
	if m.showStash {
		view.WriteString("\n")
		view.WriteString(m.renderStash())
	}

	padded := m.padToBottom(view.String())
	return padded + "\n" + m.renderStatusBar()
}

func (m Model) renderFilter() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#3C3C3C")).
		Padding(0, 1)

	label := style.Render("Filter: ")
	return label + m.input.View()
}

func (m Model) renderStash() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#3C3C3C")).
		Padding(0, 1)

	label := style.Render("Stash file (optional): ")
	return label + m.moveTarget.View()
}

func (m Model) renderStatusBar() string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#3C3C3C")).
		Foreground(lipgloss.Color("#FAFAFA")).
		Padding(0, 1)

	mode := ""
	switch {
	case m.showHelp:
		mode = "HELP"
	case m.showConfirm:
		mode = "CONFIRM DELETE"
	case m.showStash:
		mode = "STASH"
	case m.showStashView:
		mode = "STASH VIEW"
	case m.showFilter:
		mode = "FILTER"
	case m.showDetails:
		mode = "DETAILS"
	default:
		mode = "BROWSE"
	}

	filtered := len(m.list.Items())
	total := len(m.collection.Hosts)
	var hints string
	switch mode {
	case "BROWSE":
		hints = ""
	case "FILTER":
		hints = "[Enter close] [Esc clear]"
	case "STASH":
		hints = "[Enter confirm] [Esc cancel]"
	case "STASH VIEW":
		hints = "[r restore] [t back] [Esc back]"
	case "CONFIRM DELETE":
		hints = "[Enter confirm] [Esc cancel]"
	case "DETAILS":
		hints = "[Enter/Esc close]"
	case "HELP":
		hints = "[Esc close]"
	}

	page := m.list.Paginator.Page + 1
	pages := m.list.Paginator.TotalPages
	pageInfo := ""
	if pages > 1 {
		pageInfo = fmt.Sprintf(" | Page: %d/%d", page, pages)
	}

	if hints != "" {
		status := fmt.Sprintf("%s | Hosts: %d/%d%s | [? help] %s",
			mode, filtered, total, pageInfo, hints)
		return style.Render(status)
	}

	status := fmt.Sprintf("%s | Hosts: %d/%d%s | [? help]",
		mode, filtered, total, pageInfo)

	return style.Render(status)
}

func (m Model) renderHelp() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#2D2D2D")).
		Padding(1, 2)

	helpText := `
Keyboard Shortcuts:

  ↑/↓     Navigate hosts
  /       Filter hosts (live as you type)
  d       Delete selected host (with confirmation)
  s       Stash selected host into stash_hosts
  t       Toggle between known_hosts and stash_hosts view
  r       Restore selected host from stash_hosts (when in stash view)
  Enter   Confirm action / toggle host details
  Esc     Cancel current action
  ?       Toggle help
  q/Ctrl+C Quit

Host Types:
  Regular host
  Hashed host
  (n keys) Multiple keys for same host
`

	return style.Render(helpText)
}

func (m Model) renderConfirm() string {
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#A78BFA")).
		Padding(1, 2).
		Margin(1)

	var name string
	if sel := m.list.SelectedItem(); sel != nil {
		if hi, ok := sel.(hostItem); ok {
			name = hi.addressLabel
		}
	}
	if name == "" {
		name = "(no selection)"
	}

	content := fmt.Sprintf("Are you sure you want to delete ALL keys for host %q?\n\nEnter to confirm • Esc to cancel", name)
	return boxStyle.Render(content)
}

func (m Model) renderEmptyState() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Italic(true).
		Margin(1)

	msg := "No hosts to show."
	if strings.TrimSpace(m.filterText) != "" {
		msg = "No hosts match your filter. Press Esc to clear filter."
	}
	return style.Render(msg)
}

// renderDetails shows detailed information for the currently selected host.
// It is constrained to the available window height to avoid overflowing
// off-screen in smaller terminals.
func (m Model) renderDetails() string {
	selected := m.list.SelectedItem()
	if selected == nil {
		return m.renderEmptyState()
	}

	hi, ok := selected.(hostItem)
	if !ok || len(hi.hosts) == 0 {
		return m.renderEmptyState()
	}

	type hostKey struct {
		Type    string
		Key     string
		Comment string
	}
	seen := make(map[hostKey]bool)

	var lines []string

	for _, h := range hi.hosts {
		if h == nil {
			continue
		}

		hk := hostKey{Type: h.Type, Key: h.Key, Comment: h.Comment}
		if seen[hk] {
			continue
		}
		seen[hk] = true

		if h.IsHashed && h.HashValue != "" {
			lines = append(lines, fmt.Sprintf("Hashed host: %s", h.HashValue))
		} else if len(h.Addresses) > 0 {
			lines = append(lines, fmt.Sprintf("Hosts: %s", strings.Join(h.Addresses, ", ")))
		}

		// Type
		if h.Type != "" {
			lines = append(lines, fmt.Sprintf("Type: %s", h.Type))
		}

		// Key
		if h.Key != "" {
			lines = append(lines, fmt.Sprintf("Key: %s", h.Key))
		}

		// Comment
		if h.Comment != "" {
			lines = append(lines, fmt.Sprintf("Comment: %s", h.Comment))
		}

		lines = append(lines, "")
	}

	content := strings.TrimSpace(strings.Join(lines, "\n"))
	if content == "" {
		content = "No details available for this host."
	}

	maxHeight := m.height
	if maxHeight <= 0 {
		maxHeight = 24
	}
	maxLines := maxHeight - 3
	if maxLines < 3 {
		maxLines = 3
	}

	maxWidth := m.width
	if maxWidth <= 0 {
		maxWidth = 80
	}

	wrapWidth := maxWidth - 4
	if wrapWidth < 10 {
		wrapWidth = 10
	}

	rawLines := strings.Split(content, "\n")
	wrapped := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}

		current := ""
		for _, r := range line {
			next := current + string(r)
			if lipgloss.Width(next) > wrapWidth {
				// Flush current line and start a new one with this rune.
				if current != "" {
					wrapped = append(wrapped, current)
				}
				current = string(r)
				// If a single rune is somehow wider than wrapWidth, still append it
				// to avoid an infinite loop.
				if lipgloss.Width(current) > wrapWidth {
					wrapped = append(wrapped, current)
					current = ""
				}
			} else {
				current = next
			}
		}
		if current != "" {
			wrapped = append(wrapped, current)
		}
	}

	if len(wrapped) > maxLines {
		wrapped = wrapped[:maxLines]
		wrapped = append(wrapped, "… (truncated)")
	}

	content = strings.Join(wrapped, "\n")

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#A78BFA")).
		Padding(1, 2)

	return boxStyle.Render(content)
}

func (m *Model) loadStash() error {
	stashPath := m.collection.StashFilePath()
	if stashPath == "" {
		return fmt.Errorf("stash path not available")
	}

	stashCol, err := knownhosts.ParseKnownHosts(stashPath)
	if err != nil {
		return err
	}

	m.collection = stashCol
	m.filterText = ""
	m.rebuildList()
	m.list.Title = "SSH Known Hosts Manager (stash_hosts)"
	return nil
}

func (m *Model) reloadKnownHosts() {
	knownPath := m.baseKnownHostsPath
	if knownPath == "" {
		knownPath = m.collection.File
	}
	if knownPath == "" {
		// Fallback: if no known base path, assume ~/.ssh/known_hosts
		knownPath = filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	}

	col, err := knownhosts.ParseKnownHosts(knownPath)
	if err != nil {
		m.status = fmt.Sprintf("Error reloading known_hosts: %v", err)
		return
	}

	m.collection = col
	m.filterText = ""
	m.rebuildList()
	m.list.Title = "SSH Known Hosts Manager (known_hosts)"
}

func (m *Model) restoreSelectedFromStash() tea.Cmd {
	selected := m.list.SelectedItem()
	if selected == nil {
		m.status = "No host selected in stash"
		return nil
	}

	hi, ok := selected.(hostItem)
	if !ok {
		m.status = "Invalid selection"
		return nil
	}

	// Interpret addressLabel as the address key in stash_hosts.
	address := hi.addressLabel

	// Derive known_hosts path from current stash file path.
	stashPath := m.collection.File
	if stashPath == "" {
		stashPath = m.collection.StashFilePath()
	}
	if stashPath == "" {
		m.status = "Stash path not available"
		return nil
	}

	dir := filepath.Dir(stashPath)
	knownPath := filepath.Join(dir, "known_hosts")

	mainCol, err := knownhosts.ParseKnownHosts(knownPath)
	if err != nil {
		m.status = fmt.Sprintf("Error parsing known_hosts: %v", err)
		return nil
	}

	// Use UnstashAddress on a collection bound to known_hosts.
	mainCol.File = knownPath
	if err := mainCol.UnstashAddress(address); err != nil {
		m.status = fmt.Sprintf("Error restoring host: %v", err)
		return nil
	}

	// After successful restore, reload stash view
	if err := m.loadStash(); err != nil {
		m.status = fmt.Sprintf("Restored, but failed to reload stash: %v", err)
		return nil
	}

	m.status = fmt.Sprintf("Restored host %s from stash to known_hosts", address)
	return nil
}

func (m *Model) deleteSelectedHost() tea.Cmd {
	selected := m.list.SelectedItem()
	if selected == nil {
		m.status = "No host selected"
		return nil
	}

	selectedItem := selected.(hostItem)

	if err := m.collection.RemoveAllHosts(selectedItem.addressLabel); err != nil {
		m.status = fmt.Sprintf("Error: %v", err)
		return nil
	}

	if err := m.collection.Save(); err != nil {
		m.status = fmt.Sprintf("Error saving: %v", err)
		return nil
	}

	// Refresh the list respecting current filter
	m.rebuildList()

	m.status = fmt.Sprintf("Deleted all keys for host: %s", selectedItem.addressLabel)
	return nil
}

func (m *Model) stashSelectedHost() tea.Cmd {
	selected := m.list.SelectedItem()
	if selected == nil {
		m.status = "No host selected"
		return nil
	}

	hi, ok := selected.(hostItem)
	if !ok {
		m.status = "Invalid selection"
		return nil
	}

	targetFile := m.moveTarget.Value()
	if targetFile == "" {
		targetFile = m.collection.StashFilePath()
	}
	if targetFile == "" {
		m.status = "Stash path not available"
		return nil
	}

	if err := m.collection.StashAddressWithPath(hi.addressLabel, targetFile); err != nil {
		m.status = fmt.Sprintf("Error stashing host: %v", err)
		return nil
	}

	// Refresh the list respecting current filter (host removed from known_hosts)
	m.rebuildList()

	count := len(hi.hosts)
	if count <= 0 {
		m.status = fmt.Sprintf("Stashed host to: %s", targetFile)
	} else {
		m.status = fmt.Sprintf("Stashed %d key(s) for %s to: %s", count, hi.addressLabel, targetFile)
	}
	return nil
}
