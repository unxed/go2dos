package ui

import "image"

// Cell represents a single character in the terminal.
type Cell struct {
	Rune       rune
	Foreground uint8
	Background uint8
}

// Drawable is an interface for anything that can be drawn to the screen.
type Drawable interface {
	Draw(buf *Buffer) image.Rectangle
	SetBounds(bounds image.Rectangle)
	Bounds() image.Rectangle
}

// Component is a base component that all UI elements embed.
type Component struct {
	bounds image.Rectangle
}

// SetBounds sets the bounding rectangle for the component.
func (c *Component) SetBounds(bounds image.Rectangle) {
	c.bounds = bounds
}

// Bounds returns the bounding rectangle for the component.
func (c *Component) Bounds() image.Rectangle {
	return c.bounds
}

// Draw is a stub implementation. Subclasses should override.
func (c *Component) Draw(buf *Buffer) image.Rectangle {
	return c.bounds
}
