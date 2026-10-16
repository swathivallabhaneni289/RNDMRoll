package social

// Facts are what decides whether a viewer may see one entry.
type Facts struct {
	// IsOwner: the viewer wrote the entry.
	IsOwner bool
	// Follows: the viewer follows the owner with an accepted follow.
	Follows bool
	// Blocked: either of the two blocked the other.
	Blocked bool
	// Archived: the owner put the entry away.
	Archived bool
	// ViewerPostedDay: the viewer has a posted day (any source) for the
	// entry's local_date. The gate reads posted days, never entries, so a
	// deleted post keeps its day open and a skip never opens one.
	ViewerPostedDay bool
}

// CanView is THE rule. The owner always sees their entry. Anyone else sees
// it only when they follow the owner (accepted), nobody blocked anybody, the
// entry is not archived, and they posted that day themselves. Phase 6 adds
// the public branch here and nowhere else.
func CanView(f Facts) bool {
	if f.IsOwner {
		return true
	}
	if f.Blocked || f.Archived {
		return false
	}
	return f.Follows && f.ViewerPostedDay
}

// CanReact is CanView for anyone but the owner.
func CanReact(f Facts) bool {
	return CanView(f) && !f.IsOwner
}

// CanComment is CanView with the owner's comments switch on.
func CanComment(f Facts, commentsOff bool) bool {
	return CanView(f) && !commentsOff
}

// CanDeleteComment: the comment's writer or the post's owner.
func CanDeleteComment(isWriter, isOwner bool) bool {
	return isWriter || isOwner
}
