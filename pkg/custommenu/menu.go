package custommenu

import (
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Colors for the menu
var (
	MenuBackground     = color.RGBA{R: 30, G: 30, B: 30, A: 255}
	MenuHoverColor     = color.RGBA{R: 0, G: 120, B: 215, A: 255} // Windows blue
	MenuTextColor      = color.White
	MenuDisabledColor  = color.RGBA{R: 128, G: 128, B: 128, A: 255}
	MenuSeparatorColor = color.RGBA{R: 60, G: 60, B: 60, A: 255}
)

// MenuItem represents a single menu item
type MenuItem struct {
	widget.BaseWidget
	Text        string
	Tooltip     string
	Disabled    bool
	IsSeparator bool
	OnTapped    func()

	hovered    bool
	background *canvas.Rectangle
	label      *canvas.Text
	mu         sync.Mutex
}

// NewMenuItem creates a new menu item
func NewMenuItem(text string, onTapped func()) *MenuItem {
	item := &MenuItem{
		Text:     text,
		OnTapped: onTapped,
	}
	item.ExtendBaseWidget(item)
	return item
}

// NewDisabledMenuItem creates a disabled (info) menu item
func NewDisabledMenuItem(text string) *MenuItem {
	item := &MenuItem{
		Text:     text,
		Disabled: true,
	}
	item.ExtendBaseWidget(item)
	return item
}

// NewSeparator creates a separator
func NewSeparator() *MenuItem {
	item := &MenuItem{
		IsSeparator: true,
	}
	item.ExtendBaseWidget(item)
	return item
}

// CreateRenderer implements fyne.Widget
func (m *MenuItem) CreateRenderer() fyne.WidgetRenderer {
	if m.IsSeparator {
		sep := canvas.NewRectangle(MenuSeparatorColor)
		sep.SetMinSize(fyne.NewSize(200, 1))
		return widget.NewSimpleRenderer(container.NewPadded(sep))
	}

	m.background = canvas.NewRectangle(MenuBackground)
	m.label = canvas.NewText(m.Text, MenuTextColor)
	m.label.TextSize = 13

	if m.Disabled {
		m.label.Color = MenuDisabledColor
	}

	content := container.NewStack(
		m.background,
		container.NewPadded(m.label),
	)

	return widget.NewSimpleRenderer(content)
}

// MouseIn implements desktop.Hoverable
func (m *MenuItem) MouseIn(_ *desktop.MouseEvent) {
	if m.Disabled || m.IsSeparator {
		return
	}
	m.mu.Lock()
	m.hovered = true
	m.mu.Unlock()

	if m.background != nil {
		m.background.FillColor = MenuHoverColor
		m.background.Refresh()
	}
}

// MouseOut implements desktop.Hoverable
func (m *MenuItem) MouseOut() {
	if m.Disabled || m.IsSeparator {
		return
	}
	m.mu.Lock()
	m.hovered = false
	m.mu.Unlock()

	if m.background != nil {
		m.background.FillColor = MenuBackground
		m.background.Refresh()
	}
}

// MouseMoved implements desktop.Hoverable
func (m *MenuItem) MouseMoved(_ *desktop.MouseEvent) {}

// Tapped implements fyne.Tappable
func (m *MenuItem) Tapped(_ *fyne.PointEvent) {
	if m.Disabled || m.IsSeparator || m.OnTapped == nil {
		return
	}
	m.OnTapped()
}

// MinSize returns the minimum size of the menu item
func (m *MenuItem) MinSize() fyne.Size {
	if m.IsSeparator {
		return fyne.NewSize(200, 8)
	}
	return fyne.NewSize(280, 28)
}

// Cursor returns the cursor for the menu item
func (m *MenuItem) Cursor() desktop.Cursor {
	if m.Disabled || m.IsSeparator {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

// SetText updates the menu item text
func (m *MenuItem) SetText(text string) {
	m.Text = text
	if m.label != nil {
		m.label.Text = text
		m.label.Refresh()
	}
}

// PopupMenu is a custom styled popup menu
type PopupMenu struct {
	Window  fyne.Window
	Items   []*MenuItem
	app     fyne.App
	visible bool
}

// NewPopupMenu creates a new popup menu
func NewPopupMenu(app fyne.App, title string) *PopupMenu {
	w := app.NewWindow(title)
	w.SetPadded(false)

	menu := &PopupMenu{
		Window: w,
		Items:  make([]*MenuItem, 0),
		app:    app,
	}

	return menu
}

// AddItem adds a menu item
func (p *PopupMenu) AddItem(text string, onTapped func()) *MenuItem {
	item := NewMenuItem(text, func() {
		if onTapped != nil {
			onTapped()
		}
		p.Hide()
	})
	p.Items = append(p.Items, item)
	return item
}

// AddDisabledItem adds a disabled info item
func (p *PopupMenu) AddDisabledItem(text string) *MenuItem {
	item := NewDisabledMenuItem(text)
	p.Items = append(p.Items, item)
	return item
}

// AddSeparator adds a separator
func (p *PopupMenu) AddSeparator() {
	p.Items = append(p.Items, NewSeparator())
}

// Build finalizes the menu layout
func (p *PopupMenu) Build() {
	bg := canvas.NewRectangle(MenuBackground)

	vbox := container.NewVBox()
	for _, item := range p.Items {
		vbox.Add(item)
	}

	content := container.NewStack(bg, container.NewPadded(vbox))
	p.Window.SetContent(content)
	p.Window.Resize(fyne.NewSize(300, float32(len(p.Items)*30+20)))
}

// Show displays the menu at the given position
func (p *PopupMenu) Show(x, y int) {
	// Position window near cursor
	p.Window.Resize(p.Window.Content().MinSize())
	p.Window.Show()
	p.visible = true
}

// Hide hides the menu
func (p *PopupMenu) Hide() {
	p.Window.Hide()
	p.visible = false
}

// Toggle shows or hides the menu
func (p *PopupMenu) Toggle() {
	if p.visible {
		p.Hide()
	} else {
		p.Show(0, 0)
	}
}

// IsVisible returns whether the menu is currently visible
func (p *PopupMenu) IsVisible() bool {
	return p.visible
}

// CustomTheme is a dark theme for the menu
type CustomTheme struct{}

func (t *CustomTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return MenuBackground
	case theme.ColorNameButton:
		return MenuBackground
	case theme.ColorNameForeground:
		return MenuTextColor
	default:
		return theme.DefaultTheme().Color(n, v)
	}
}

func (t *CustomTheme) Font(s fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(s)
}

func (t *CustomTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (t *CustomTheme) Size(n fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(n)
}
