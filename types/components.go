package types

// Component types.
const (
	ComponentActionRow         = 1
	ComponentButton            = 2
	ComponentStringSelect      = 3
	ComponentTextInput         = 4
	ComponentUserSelect        = 5
	ComponentRoleSelect        = 6
	ComponentMentionableSelect = 7
	ComponentChannelSelect     = 8
	ComponentSection           = 9
	ComponentTextDisplay       = 10
	ComponentThumbnail         = 11
	ComponentMediaGallery      = 12
	ComponentFile              = 13
	ComponentSeparator         = 14
	ComponentContainer         = 17
	ComponentLabel             = 18
)

// Button styles.
const (
	ButtonPrimary   = 1
	ButtonSecondary = 2
	ButtonSuccess   = 3
	ButtonDanger    = 4
	ButtonLink      = 5
	ButtonPremium   = 6
)

// Component is a message component: an action row, button, select menu or one
// of the layout components (section, container, text display, ...). Layout
// components nest other components in Components (and Accessory for sections).
type Component struct {
	Type     int    `json:"type"`
	ID       int    `json:"id,omitempty"`
	CustomID string `json:"custom_id,omitempty"`

	// Buttons
	Style    int             `json:"style,omitempty"`
	Label    string          `json:"label,omitempty"`
	Emoji    *ComponentEmoji `json:"emoji,omitempty"`
	URL      string          `json:"url,omitempty"`
	SKUID    string          `json:"sku_id,omitempty"`
	Disabled bool            `json:"disabled,omitempty"`
	Hash     string          `json:"hash,omitempty"`

	// Select menus
	Placeholder string         `json:"placeholder,omitempty"`
	MinValues   *int           `json:"min_values,omitempty"`
	MaxValues   *int           `json:"max_values,omitempty"`
	Options     []SelectOption `json:"options,omitempty"`

	// Text display
	Content string `json:"content,omitempty"`

	// Layout
	Components  []Component `json:"components,omitempty"`
	Accessory   *Component  `json:"accessory,omitempty"`
	AccentColor *int        `json:"accent_color,omitempty"`
	Spoiler     bool        `json:"spoiler,omitempty"`
}

// ComponentEmoji is the emoji shown on a button or select option.
type ComponentEmoji struct {
	Name     string `json:"name,omitempty"`
	ID       string `json:"id,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// SelectOption is one choice of a string select menu.
type SelectOption struct {
	Label       string          `json:"label"`
	Value       string          `json:"value"`
	Description string          `json:"description,omitempty"`
	Emoji       *ComponentEmoji `json:"emoji,omitempty"`
	Default     bool            `json:"default,omitempty"`
}

// IsSelectMenu reports whether the component is any kind of select menu.
func (c *Component) IsSelectMenu() bool {
	switch c.Type {
	case ComponentStringSelect, ComponentUserSelect, ComponentRoleSelect,
		ComponentMentionableSelect, ComponentChannelSelect:
		return true
	}
	return false
}

// Walk calls fn for the component and, depth first, every component nested in it.
func (c *Component) Walk(fn func(*Component)) {
	fn(c)
	for i := range c.Components {
		c.Components[i].Walk(fn)
	}
	if c.Accessory != nil {
		c.Accessory.Walk(fn)
	}
}

// MessageComponent is kept for backwards compatibility; use Component.
type MessageComponent = Component

// Buttons is kept for backwards compatibility; use Component.
type Buttons = Component

// ButtonEmoji is kept for backwards compatibility; use ComponentEmoji.
type ButtonEmoji = ComponentEmoji
