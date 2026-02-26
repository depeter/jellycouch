package ui

// ScrollState provides reusable vertical scroll tracking with smooth animation.
// Embed this struct in screens that need scrollable content.
type ScrollState struct {
	ScrollY       float64
	TargetScrollY float64
}

// HandleMouseWheel updates the target scroll position from mouse wheel input.
// Call this from Update() with the vertical wheel delta.
func (s *ScrollState) HandleMouseWheel() {
	_, wy := MouseWheelDelta()
	if wy != 0 {
		s.TargetScrollY -= wy * ScrollWheelSpeed
		if s.TargetScrollY < 0 {
			s.TargetScrollY = 0
		}
	}
}

// Animate performs smooth scroll interpolation. Call this from Draw().
func (s *ScrollState) Animate() {
	s.ScrollY = Lerp(s.ScrollY, s.TargetScrollY, ScrollAnimSpeed)
}

// Reset sets scroll position back to top.
func (s *ScrollState) Reset() {
	s.ScrollY = 0
	s.TargetScrollY = 0
}

// EnsureRowVisible scrolls to make the given row index visible in a grid layout.
// gridBaseY is the top of the grid area (without scroll offset applied).
// viewHeight is the visible viewport height.
func (s *ScrollState) EnsureRowVisible(row int, gridBaseY, viewHeight float64) {
	rowTop := gridBaseY + float64(row)*GridRowHeight
	rowBottom := rowTop + GridRowHeight

	// Scroll down if row is below viewport
	if rowBottom > viewHeight+s.TargetScrollY {
		s.TargetScrollY = rowBottom - viewHeight
	}
	// Scroll up if row is above viewport
	if rowTop < s.TargetScrollY {
		s.TargetScrollY = rowTop - gridBaseY
		if s.TargetScrollY < 0 {
			s.TargetScrollY = 0
		}
	}
}

// EnsureSectionVisible scrolls to make a home-style section visible.
// sectionIdx is the 0-based section index.
// baseY is the starting Y position of the first section.
func (s *ScrollState) EnsureSectionVisible(sectionIdx int, baseY, viewHeight float64) {
	sectionTop := baseY + float64(sectionIdx)*SectionFullHeight
	sectionBottom := sectionTop + SectionFullHeight

	if sectionBottom > viewHeight+s.TargetScrollY {
		s.TargetScrollY = sectionBottom - viewHeight
	}
	if sectionTop < s.TargetScrollY {
		s.TargetScrollY = sectionTop - baseY
		if s.TargetScrollY < 0 {
			s.TargetScrollY = 0
		}
	}
}
