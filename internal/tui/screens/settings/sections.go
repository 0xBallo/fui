package settings

import (
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/0xballo/fui/internal/domain"
	"github.com/0xballo/fui/internal/module"
	"github.com/0xballo/fui/internal/tui/screens"
	"github.com/0xballo/fui/internal/tui/screens/settings/lists"
)

// builtinSections is the number of shell-owned sections (Modules, Appearance); sub-view sections
// follow them.
const builtinSections = len(sectionNames)

// Lists are the ports of the CRUD sub-views the shell embeds as extra sections. A nil port omits
// its section; Recurrences also needs the recurring module enabled.
type Lists struct {
	Locale      domain.Locale
	Categories  lists.CategoriesUseCases
	Tags        lists.TagsUseCases
	Rules       lists.RulesUseCases
	Recurrences lists.RecurrencesUseCases
	Profiles    lists.ProfilesConfig
	Assets      lists.AssetsUseCases
}

// sub is one embedded CRUD sub-view and its section tab label.
type sub struct {
	name string
	scr  screens.Screen
}

// SetLists enables the CRUD sub-view sections over l.
func (m *Model) SetLists(l Lists) {
	m.lists = l
	m.rebuildSubs()
}

// rebuildSubs recreates the sub-views for the current theme and enabled modules, and keeps the
// active section in range (a module change can remove the Recurrences tab).
func (m *Model) rebuildSubs() {
	m.subs = m.subs[:0]
	l := m.lists
	if l.Categories != nil {
		m.subs = append(m.subs, sub{"Categories", lists.NewCategories(l.Categories, m.th)})
	}
	if l.Tags != nil {
		m.subs = append(m.subs, sub{"Tags", lists.NewTags(l.Tags, m.th)})
	}
	if l.Rules != nil {
		m.subs = append(m.subs, sub{"Rules", lists.NewRules(l.Rules, m.th)})
	}
	if l.Recurrences != nil && m.enabled.Has(module.Recurring) {
		m.subs = append(m.subs, sub{"Recurrences", lists.NewRecurrences(l.Recurrences, m.enabled, l.Locale, m.th)})
	}
	if l.Assets != nil && m.enabled.Has(module.Assets) {
		m.subs = append(m.subs, sub{"Assets", lists.NewAssets(l.Assets, m.enabled, l.Locale, m.th)})
	}
	if l.Profiles != nil {
		m.subs = append(m.subs, sub{"Import profiles", lists.NewProfiles(l.Profiles, m.th)})
	}
	if m.height > 0 {
		for _, s := range m.subs {
			s.scr.SetSize(m.width, m.height)
		}
	}
	if int(m.section) >= builtinSections+len(m.subs) {
		m.section = sectionModules
	}
}

// activeSub returns the sub-view of the active section, or nil for Modules and Appearance.
func (m *Model) activeSub() screens.Screen {
	if i := int(m.section) - builtinSections; i >= 0 && i < len(m.subs) {
		return m.subs[i].scr
	}
	return nil
}

func (m *Model) sectionLabels() []string {
	labels := append([]string{}, sectionNames[:]...)
	for _, s := range m.subs {
		labels = append(labels, s.name)
	}
	return labels
}

// nextSection moves to the next section, dropping an unsaved theme preview, and loads a sub-view
// when it is entered.
func (m *Model) nextSection() tea.Cmd {
	restore := m.restoreTheme()
	m.section = (m.section + 1) % section(builtinSections+len(m.subs))
	if i := int(m.section) - builtinSections; i >= 0 && i < len(m.subs) {
		return tea.Batch(restore, tagCmd(m.subs[i].name, m.subs[i].scr.Init()))
	}
	return restore
}

// sectionMsg is the result of a command started by a sub-view, tagged with that sub-view's name
// so it reaches it even when the user has moved to another section before it arrives.
type sectionMsg struct {
	section string
	msg     tea.Msg
}

// tagCmd wraps c so its result is a sectionMsg for the named section. Messages meant for the root
// router (errors, status, navigation, quit) pass through untouched, and batches are tagged per
// command.
func tagCmd(section string, c tea.Cmd) tea.Cmd {
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		switch msg := c().(type) {
		case nil:
			return nil
		case tea.BatchMsg:
			tagged := make(tea.BatchMsg, len(msg))
			for i, inner := range msg {
				tagged[i] = tagCmd(section, inner)
			}
			return tagged
		case screens.ErrorMsg, screens.StatusMsg, screens.SwitchScreenMsg, tea.QuitMsg:
			return msg
		default:
			return sectionMsg{section: section, msg: msg}
		}
	}
}

// updateSub forwards msg to the active sub-view; a sectionMsg goes to the section it targets
// instead, whichever one is active.
func (m *Model) updateSub(msg tea.Msg) (screens.Screen, tea.Cmd) {
	i := int(m.section) - builtinSections
	if tagged, ok := msg.(sectionMsg); ok {
		i = slices.IndexFunc(m.subs, func(s sub) bool { return s.name == tagged.section })
		msg = tagged.msg
	}
	if i < 0 || i >= len(m.subs) {
		return m, nil
	}
	next, cmd := m.subs[i].scr.Update(msg)
	m.subs[i].scr = next
	return m, tagCmd(m.subs[i].name, cmd)
}

func (m *Model) subKeyMap() []key.Binding {
	if s := m.activeSub(); s != nil {
		return append(m.km.Bindings(), s.KeyMap()...)
	}
	return m.km.Bindings()
}
