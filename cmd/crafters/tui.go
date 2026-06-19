package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	opencrafters "github.com/Rohithgilla12/open-crafters"
	"github.com/Rohithgilla12/open-crafters/internal/harness"
	"github.com/Rohithgilla12/open-crafters/internal/progress"
)

// program is the running tea.Program, so the grading goroutine can stream
// stage events back into the UI via Send.
var program *tea.Program

func runTUI() int {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println("crafters: the dashboard needs an interactive terminal.")
		fmt.Println("Try:  crafters list  ·  crafters start wal  ·  crafters test")
		return 0
	}
	program = tea.NewProgram(newModel(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "crafters:", err)
		return 1
	}
	return 0
}

type screen int

const (
	screenHome screen = iota
	screenLang
	screenName
	screenGrading
)

type stageState int

const (
	statePending stageState = iota
	stateRunning
	statePassed
	stateFailed
)

type row struct {
	slug, name  string
	stages      int
	passed      int
	solutionDir string // "" if not started in this directory
}

type model struct {
	rows   []row
	cursor int
	scr    screen

	// selection
	sel        row
	ch         harness.Challenge
	lang       int
	langChoice string
	nameInput  textinput.Model
	defaultName string

	// grading
	spinner  spinner.Model
	stages   []harness.Stage
	state    []stageState
	failSlug string
	failMsg  string
	done     bool
	gradeErr error
	banner   string

	width int
}

var langChoices = []string{"python", "go", "typescript"}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	selStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	failStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	bannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

// diffTag renders a colored difficulty label for the TUI.
func diffTag(d string) string {
	switch d {
	case "easy":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("easy  ")
	case "medium":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("medium")
	case "hard":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("hard  ")
	}
	return d
}

func newModel() *model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return &model{rows: buildRows(), scr: screenHome, spinner: sp}
}

func buildRows() []row {
	var rows []row
	for _, slug := range orderedSlugs() {
		ch := challenges[slug]
		dir := findSolution(slug)
		rows = append(rows, row{
			slug:        slug,
			name:        ch.Name,
			stages:      len(ch.Stages),
			passed:      solutionPassed(ch, dir),
			solutionDir: dir,
		})
	}
	return rows
}

// findSolution looks in the current directory for a child solution scaffolded
// for the given challenge.
func findSolution(slug string) string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(e.Name(), ".open-crafters", "challenge"))
		if err == nil && strings.TrimSpace(string(b)) == slug {
			return e.Name()
		}
	}
	return ""
}

func solutionPassed(ch harness.Challenge, dir string) int {
	if dir == "" {
		return 0
	}
	prog, err := progress.Load(progress.PathFor(filepath.Join(dir, "your_program.sh")))
	if err != nil {
		return 0
	}
	return passedCount(ch, prog)
}

// --- messages streamed from the grading goroutine ---

type stageStartMsg struct{ slug string }
type stageEndMsg struct {
	slug string
	ok   bool
	msg  string
}
type gradeDoneMsg struct {
	err error
	dir string
}
type scaffoldErrMsg struct{ err error }

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if m.scr == screenGrading && !m.done {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case stageStartMsg:
		m.setState(msg.slug, stateRunning)
		return m, nil

	case stageEndMsg:
		if msg.ok {
			m.setState(msg.slug, statePassed)
		} else {
			m.setState(msg.slug, stateFailed)
			m.failSlug = msg.slug
			m.failMsg = msg.msg
		}
		return m, nil

	case gradeDoneMsg:
		m.done = true
		m.gradeErr = msg.err
		m.sel.solutionDir = msg.dir
		m.refreshRow(m.sel.slug, msg.dir)
		return m, nil

	case scaffoldErrMsg:
		m.banner = "could not start: " + msg.err.Error()
		m.scr = screenHome
		return m, nil
	}
	// Keep the text input (and its blinking cursor) alive on the name screen.
	if m.scr == screenName {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	}

	switch m.scr {
	case screenHome:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter":
			m.banner = ""
			m.sel = m.rows[m.cursor]
			m.ch = challenges[m.sel.slug]
			if m.sel.solutionDir != "" {
				return m, m.beginGrading(m.sel.solutionDir, "", "")
			}
			m.scr = screenLang
			m.lang = 0
		}

	case screenLang:
		switch msg.String() {
		case "esc", "q":
			m.scr = screenHome
		case "up", "k":
			if m.lang > 0 {
				m.lang--
			}
		case "down", "j":
			if m.lang < len(langChoices)-1 {
				m.lang++
			}
		case "enter":
			m.langChoice = langChoices[m.lang]
			return m, m.enterNameScreen()
		}

	case screenName:
		switch msg.String() {
		case "esc":
			m.scr = screenLang
		case "enter":
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				name = m.defaultName
			}
			return m, m.beginGrading("", m.langChoice, name)
		default:
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}

	case screenGrading:
		if !m.done {
			return m, nil // ignore input mid-run (except ctrl+c above)
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "enter", "esc":
			m.rows = buildRows()
			m.scr = screenHome
		}
	}
	return m, nil
}

// enterNameScreen sets up the directory-name prompt, pre-filled with the
// default so a learner can just press enter.
func (m *model) enterNameScreen() tea.Cmd {
	m.scr = screenName
	m.defaultName = "my-" + strings.TrimPrefix(m.sel.slug, "build-your-own-")
	ti := textinput.New()
	ti.Placeholder = m.defaultName
	ti.SetValue(m.defaultName)
	ti.CharLimit = 64
	ti.Width = 36
	ti.Focus()
	ti.CursorEnd()
	m.nameInput = ti
	return textinput.Blink
}

// beginGrading enters the grading screen and launches the run. dir is set when
// resuming an existing solution; otherwise (lang, name) drive a fresh scaffold.
func (m *model) beginGrading(dir, lang, name string) tea.Cmd {
	m.scr = screenGrading
	m.stages = m.ch.Stages
	m.state = make([]stageState, len(m.stages))
	m.failSlug, m.failMsg = "", ""
	m.done, m.gradeErr = false, nil

	slug := m.sel.slug
	ch := m.ch
	start := func() tea.Msg {
		if dir == "" {
			d := name
			if d == "" {
				d = "my-" + strings.TrimPrefix(slug, "build-your-own-")
			}
			if err := opencrafters.Scaffold(slug, lang, d); err != nil {
				return scaffoldErrMsg{err}
			}
			go grade(ch, d)
		} else {
			go grade(ch, dir)
		}
		return nil
	}
	return tea.Batch(m.spinner.Tick, start)
}

// grade runs the resume set for a solution, streaming events to the program.
func grade(ch harness.Challenge, dir string) {
	programPath, _ := filepath.Abs(filepath.Join(dir, "your_program.sh"))
	progressPath := progress.PathFor(programPath)
	prog, _ := progress.Load(progressPath)
	target := ""
	if next := firstUnpassed(ch, prog); next != nil {
		target = next.Slug
	}
	err := harness.Run(ch, harness.RunOptions{
		TargetSlug:  target,
		ProgramPath: programPath,
		Logf:        func(string, ...any) {},
		OnStageStart: func(s harness.Stage) {
			program.Send(stageStartMsg{s.Slug})
		},
		OnStageEnd: func(s harness.Stage, e error, _ time.Duration) {
			msg := ""
			if e != nil {
				msg = e.Error()
			}
			program.Send(stageEndMsg{slug: s.Slug, ok: e == nil, msg: msg})
		},
		OnStagePass: func(s harness.Stage) {
			prog.MarkPassed(ch.Slug, s.Slug)
			_ = progress.Save(progressPath, prog)
		},
	})
	program.Send(gradeDoneMsg{err: err, dir: dir})
}

func (m *model) setState(slug string, st stageState) {
	for i := range m.stages {
		if m.stages[i].Slug == slug {
			m.state[i] = st
			return
		}
	}
}

func (m *model) refreshRow(slug, dir string) {
	for i := range m.rows {
		if m.rows[i].slug == slug {
			m.rows[i].solutionDir = dir
			m.rows[i].passed = solutionPassed(challenges[slug], dir)
		}
	}
}

func (m *model) View() string {
	switch m.scr {
	case screenLang:
		return m.viewLang()
	case screenName:
		return m.viewName()
	case screenGrading:
		return m.viewGrading()
	default:
		return m.viewHome()
	}
}

func (m *model) viewHome() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("open-crafters") + "  build-your-own-X for serious infrastructure\n\n")
	for i, r := range m.rows {
		name := fmt.Sprintf("%-32s", r.name)
		base := fmt.Sprintf("%s %2d/%-2d  %s", name, r.passed, r.stages, bar(r.passed, r.stages, 12))
		if i == m.cursor {
			b.WriteString(selStyle.Render("▸ " + base))
		} else {
			b.WriteString("  " + base)
		}
		if r.solutionDir != "" {
			b.WriteString("  " + dimStyle.Render("./"+r.solutionDir))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.banner != "" {
		b.WriteString(bannerStyle.Render(m.banner) + "\n\n")
	}
	b.WriteString(helpStyle.Render("↑/↓ select · enter: start/resume · q: quit"))
	return b.String()
}

func (m *model) viewName() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Start "+m.sel.name) + dimStyle.Render("  ·  "+m.langChoice) + "\n")
	b.WriteString(dimStyle.Render("Name your solution directory:") + "\n\n")
	b.WriteString("  " + m.nameInput.View() + "\n\n")
	b.WriteString(helpStyle.Render("enter: scaffold & grade · esc: back"))
	return b.String()
}

func (m *model) viewLang() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Start "+m.sel.name) + "\n")
	b.WriteString(dimStyle.Render("Pick a starter language:") + "\n\n")
	for i, l := range langChoices {
		line := "  " + l
		if i == m.lang {
			line = selStyle.Render("▸ " + l)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("↑/↓ select · enter: scaffold & grade · esc: back"))
	return b.String()
}

func (m *model) viewGrading() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("grading "+m.sel.name) + "\n\n")
	for i, s := range m.stages {
		var icon string
		switch m.state[i] {
		case stateRunning:
			icon = m.spinner.View()
		case statePassed:
			icon = okStyle.Render("✓")
		case stateFailed:
			icon = failStyle.Render("✗")
		default:
			icon = dimStyle.Render("·")
		}
		label := fmt.Sprintf("%-18s %-24s", s.Slug, s.Name)
		if m.state[i] == statePending {
			label = dimStyle.Render(label)
		}
		b.WriteString("  " + icon + "  " + diffTag(s.Difficulty) + "  " + label + "\n")
	}
	b.WriteString("\n")
	if m.failMsg != "" {
		b.WriteString(failStyle.Render("✗ "+m.failSlug) + "\n")
		b.WriteString(wrap("  "+m.failMsg, max(m.width-2, 40)) + "\n\n")
	}
	if m.done {
		if m.gradeErr == nil {
			b.WriteString(okStyle.Render("🏆 all stages passed!") + "\n\n")
		}
		b.WriteString(helpStyle.Render("enter: back to dashboard · q: quit"))
	} else {
		b.WriteString(helpStyle.Render("grading… · ctrl+c: quit"))
	}
	return b.String()
}

func bar(passed, total, width int) string {
	if total == 0 {
		return ""
	}
	filled := passed * width / total
	return okStyle.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", width-filled))
}

// wrap soft-wraps s to width columns (naive, by words).
func wrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var b strings.Builder
	line := 0
	for i, w := range words {
		if i > 0 && line+1+len(w) > width {
			b.WriteString("\n  ")
			line = 2
		} else if i > 0 {
			b.WriteString(" ")
			line++
		}
		b.WriteString(w)
		line += len(w)
	}
	return b.String()
}
