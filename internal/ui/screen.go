package ui

import "github.com/hajimehoshi/ebiten/v2"

// Screen is the interface for all UI screens (Home, Library, Detail, Search, Settings, Login).
type Screen interface {
	// Update handles input and logic. Return a non-nil ScreenTransition to change screens.
	Update() (*ScreenTransition, error)
	// Draw renders the screen.
	Draw(dst *ebiten.Image)
	// OnEnter is called when the screen becomes active.
	OnEnter()
	// OnExit is called when the screen is removed.
	OnExit()
	// Name returns the screen name for debugging.
	Name() string
}

type TransitionType int

const (
	TransitionPush TransitionType = iota
	TransitionPop
	TransitionReplace
	TransitionFocusNavBar // request navbar keyboard focus
)

type ScreenTransition struct {
	Type   TransitionType
	Screen Screen // nil for Pop and FocusNavBar
}

// ScreenManager manages a stack of screens.
type ScreenManager struct {
	stack        []Screen
	NavBar       *NavBar
	navBarActive bool
}

func NewScreenManager() *ScreenManager {
	return &ScreenManager{}
}

func (sm *ScreenManager) Push(s Screen) {
	sm.stack = append(sm.stack, s)
	s.OnEnter()
}

func (sm *ScreenManager) Pop() {
	if len(sm.stack) == 0 {
		return
	}
	top := sm.stack[len(sm.stack)-1]
	top.OnExit()
	sm.stack = sm.stack[:len(sm.stack)-1]
	if len(sm.stack) > 0 {
		sm.stack[len(sm.stack)-1].OnEnter()
	}
}

func (sm *ScreenManager) Replace(s Screen) {
	if len(sm.stack) > 0 {
		top := sm.stack[len(sm.stack)-1]
		top.OnExit()
		sm.stack[len(sm.stack)-1] = s
	} else {
		sm.stack = append(sm.stack, s)
	}
	s.OnEnter()
}

// ClearStack exits and removes all screens from the stack.
func (sm *ScreenManager) ClearStack() {
	for len(sm.stack) > 0 {
		top := sm.stack[len(sm.stack)-1]
		top.OnExit()
		sm.stack = sm.stack[:len(sm.stack)-1]
	}
}

func (sm *ScreenManager) Current() Screen {
	if len(sm.stack) == 0 {
		return nil
	}
	return sm.stack[len(sm.stack)-1]
}

func (sm *ScreenManager) Update() error {
	s := sm.Current()
	if s == nil {
		return nil
	}

	// Mouse clicks in navbar area are intercepted before the screen gets them
	if sm.NavBar != nil && s.Name() != "Login" {
		mx, my, clicked := MouseJustClicked()
		if clicked && float64(my) < NavBarHeight {
			if sm.NavBar.HandleClick(mx, my) {
				sm.navBarActive = false
				sm.NavBar.Active = false
			}
			sm.updateNavBarHighlight()
			return nil
		}
	}

	// When navbar has keyboard focus, route input to it instead of the screen
	if sm.navBarActive && sm.NavBar != nil {
		action := sm.NavBar.Update()
		if action == NavBarActionDefocus {
			sm.navBarActive = false
		}
		sm.updateNavBarHighlight()
		return nil
	}

	tr, err := s.Update()
	if err != nil {
		return err
	}
	if tr != nil {
		switch tr.Type {
		case TransitionPush:
			sm.Push(tr.Screen)
		case TransitionPop:
			sm.Pop()
		case TransitionReplace:
			sm.Replace(tr.Screen)
		case TransitionFocusNavBar:
			if sm.NavBar != nil {
				sm.navBarActive = true
				sm.NavBar.FocusFromBelow()
			}
		}
	}

	sm.updateNavBarHighlight()
	return nil
}

func (sm *ScreenManager) updateNavBarHighlight() {
	if sm.NavBar == nil {
		return
	}
	if cur := sm.Current(); cur != nil {
		sm.NavBar.ActiveScreenName = cur.Name()
	}
}

func (sm *ScreenManager) Draw(dst *ebiten.Image) {
	s := sm.Current()
	if s != nil {
		s.Draw(dst)
	}
	// Draw navbar overlay on all screens except Login
	if sm.NavBar != nil && s != nil && s.Name() != "Login" {
		sm.NavBar.Draw(dst)
	}
}

func (sm *ScreenManager) StackSize() int {
	return len(sm.stack)
}
