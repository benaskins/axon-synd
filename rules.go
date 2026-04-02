package synd

import rule "github.com/benaskins/axon-rule"

// PostCandidate holds the data needed to evaluate post lifecycle rules.
type PostCandidate struct {
	Post Post
}

type NotDraft struct{ Status PostStatus }
type NotApproved struct{ Status PostStatus }
type AlreadyDeleted struct{}
type MissingBody struct{}

func (c PostCandidate) IsDraft() rule.Verdict {
	if c.Post.Status == StatusDraft {
		return rule.Pass()
	}
	return rule.FailWith(NotDraft{Status: c.Post.Status})
}

func (c PostCandidate) IsApproved() rule.Verdict {
	if c.Post.Status == StatusApproved {
		return rule.Pass()
	}
	return rule.FailWith(NotApproved{Status: c.Post.Status})
}

func (c PostCandidate) IsNotDeleted() rule.Verdict {
	if c.Post.Status != StatusDeleted {
		return rule.Pass()
	}
	return rule.FailWith(AlreadyDeleted{})
}

func (c PostCandidate) HasBody() rule.Verdict {
	if c.Post.Body != "" {
		return rule.Pass()
	}
	return rule.FailWith(MissingBody{})
}

// CanApprove requires the post to be a draft.
var CanApprove = rule.AllOf(
	rule.New(PostCandidate.IsDraft),
)

// CanPublish requires the post to be approved.
var CanPublish = rule.AllOf(
	rule.New(PostCandidate.IsApproved),
)

// CanRevise requires the post to be a draft.
var CanRevise = rule.AllOf(
	rule.New(PostCandidate.IsDraft),
)

// CanDelete allows any non-deleted post to be deleted.
var CanDelete = rule.AllOf(
	rule.New(PostCandidate.IsNotDeleted),
)
