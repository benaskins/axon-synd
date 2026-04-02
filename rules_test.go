package synd

import (
	"testing"

	rule "github.com/benaskins/axon-rule"
)

func TestCanApprove(t *testing.T) {
	tests := []struct {
		name       string
		status     PostStatus
		expectPass bool
	}{
		{"draft can be approved", StatusDraft, true},
		{"approved cannot be approved again", StatusApproved, false},
		{"published cannot be approved", StatusPublished, false},
		{"deleted cannot be approved", StatusDeleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := PostCandidate{Post: Post{Status: tt.status}}
			v := rule.New(PostCandidate.IsDraft).Check(c)
			if v.OK != tt.expectPass {
				t.Errorf("expected OK=%v, got OK=%v", tt.expectPass, v.OK)
			}
			if !v.OK {
				ctx, ok := v.Context.(NotDraft)
				if !ok {
					t.Fatalf("expected NotDraft context, got %T", v.Context)
				}
				if ctx.Status != tt.status {
					t.Errorf("expected status %s, got %s", tt.status, ctx.Status)
				}
			}
		})
	}
}

func TestCanPublish(t *testing.T) {
	tests := []struct {
		name       string
		status     PostStatus
		expectPass bool
	}{
		{"approved can be published", StatusApproved, true},
		{"draft cannot be published", StatusDraft, false},
		{"published cannot be published again", StatusPublished, false},
		{"deleted cannot be published", StatusDeleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := PostCandidate{Post: Post{Status: tt.status}}
			v := rule.New(PostCandidate.IsApproved).Check(c)
			if v.OK != tt.expectPass {
				t.Errorf("expected OK=%v, got OK=%v", tt.expectPass, v.OK)
			}
			if !v.OK {
				ctx, ok := v.Context.(NotApproved)
				if !ok {
					t.Fatalf("expected NotApproved context, got %T", v.Context)
				}
				if ctx.Status != tt.status {
					t.Errorf("expected status %s, got %s", tt.status, ctx.Status)
				}
			}
		})
	}
}

func TestCanRevise(t *testing.T) {
	tests := []struct {
		name       string
		status     PostStatus
		expectPass bool
	}{
		{"draft can be revised", StatusDraft, true},
		{"approved cannot be revised", StatusApproved, false},
		{"published cannot be revised", StatusPublished, false},
		{"deleted cannot be revised", StatusDeleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := PostCandidate{Post: Post{Status: tt.status}}
			v := CanRevise.Check(c)
			if v.OK != tt.expectPass {
				t.Errorf("expected OK=%v, got OK=%v", tt.expectPass, v.OK)
			}
		})
	}
}

func TestCanDelete(t *testing.T) {
	tests := []struct {
		name       string
		status     PostStatus
		expectPass bool
	}{
		{"draft can be deleted", StatusDraft, true},
		{"approved can be deleted", StatusApproved, true},
		{"published can be deleted", StatusPublished, true},
		{"deleted cannot be deleted again", StatusDeleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := PostCandidate{Post: Post{Status: tt.status}}
			v := CanDelete.Check(c)
			if v.OK != tt.expectPass {
				t.Errorf("expected OK=%v, got OK=%v", tt.expectPass, v.OK)
			}
		})
	}
}

func TestHasBody(t *testing.T) {
	c := PostCandidate{Post: Post{Body: "hello"}}
	if v := rule.New(PostCandidate.HasBody).Check(c); !v.OK {
		t.Error("expected pass for non-empty body")
	}

	c.Post.Body = ""
	v := rule.New(PostCandidate.HasBody).Check(c)
	if v.OK {
		t.Error("expected fail for empty body")
	}
	if _, ok := v.Context.(MissingBody); !ok {
		t.Errorf("expected MissingBody context, got %T", v.Context)
	}
}
