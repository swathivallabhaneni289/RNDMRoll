package spin

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Wheel limits. The Edit wheel screen enforces the same numbers and the
// server checks them again.
const (
	MinWheelEntries   = 2
	MaxWheelEntries   = 12
	MaxWheelNameRunes = 24
	MinWeight         = 1
	MaxWeight         = 10

	// MaxCategoryRows caps every row a person has in wheel_categories,
	// archived ones included, so edits cannot grow the table without bound.
	MaxCategoryRows = 500
)

// WheelInput is one category as a caller sends it: ID is nil for a new
// category and the id of one of the person's live categories otherwise.
type WheelInput struct {
	ID     *string
	Name   string
	Weight int
}

// DefaultWheel returns the starting wheel, in order: Meal 3, Movie 1, Song 3,
// Book 1, Place 2, Game 1. Movie and Book are small wedges on purpose (about
// one day in five). It is written to a person's wheel on first use.
func DefaultWheel() []WheelInput {
	return []WheelInput{
		{Name: "Meal", Weight: 3},
		{Name: "Movie", Weight: 1},
		{Name: "Song", Weight: 3},
		{Name: "Book", Weight: 1},
		{Name: "Place", Weight: 2},
		{Name: "Game", Weight: 1},
	}
}

// CleanName returns a category name as it is stored: surrounding spaces
// removed.
func CleanName(name string) string {
	return strings.TrimSpace(name)
}

// nameProblem returns "" for a usable name, else the reason. A name must
// hold at least one visible character, no control character, and be at most
// 24 characters after trimming.
func nameProblem(name string) string {
	if !utf8.ValidString(name) {
		return ReasonNameRequired
	}
	trimmed := strings.TrimSpace(name)
	visible := false
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return ReasonNameRequired
		}
		if unicode.IsGraphic(r) && !unicode.IsSpace(r) {
			visible = true
		}
	}
	if !visible {
		return ReasonNameRequired
	}
	if utf8.RuneCountInString(trimmed) > MaxWheelNameRunes {
		return ReasonNameTooLong
	}
	return ""
}

// ValidateWheel checks a whole wheel before anything is written. It returns
// nil, or a *WheelError (which matches ErrWheelInvalid) with the first
// problem found and the index of the row at fault (-1 for the list as a
// whole):
//
//   - too_few, too_many: fewer than 2 or more than 12 entries;
//   - name_required: empty, blank, no visible character or a control
//     character after trimming;
//   - name_too_long: more than 24 characters after trimming;
//   - weight_range: a weight outside 1 to 10;
//   - duplicate_name: the same name as an earlier row, ignoring capitals;
//   - duplicate_id: the same category id as an earlier row.
//
// Rows are checked in order, and within a row in the order above.
func ValidateWheel(list []WheelInput) error {
	if len(list) < MinWheelEntries {
		return &WheelError{Reason: ReasonTooFew, Index: -1}
	}
	if len(list) > MaxWheelEntries {
		return &WheelError{Reason: ReasonTooMany, Index: -1}
	}
	names := make(map[string]bool, len(list))
	ids := make(map[string]bool, len(list))
	for i, e := range list {
		if reason := nameProblem(e.Name); reason != "" {
			return &WheelError{Reason: reason, Index: i}
		}
		if e.Weight < MinWeight || e.Weight > MaxWeight {
			return &WheelError{Reason: ReasonWeightRange, Index: i}
		}
		key := strings.ToLower(CleanName(e.Name))
		if names[key] {
			return &WheelError{Reason: ReasonDuplicateName, Index: i}
		}
		names[key] = true
		if e.ID != nil {
			id := strings.ToLower(*e.ID)
			if ids[id] {
				return &WheelError{Reason: ReasonDuplicateID, Index: i}
			}
			ids[id] = true
		}
	}
	return nil
}
