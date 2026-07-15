package whatsapp_usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// newGroupUsecase builds a usecase carrying only the config gate flags plus a
// cache — enough to exercise the group guards, which must all fire before the
// (nil) manager is touched. pacer is nil (pace is a no-op), so the guards, not
// pacing, are what these tests assert.
func newGroupUsecase(cfg *config.Config) *WhatsappMessageUsecase {
	return &WhatsappMessageUsecase{
		config:      cfg,
		queryCache:  newTTLCache(time.Minute),
		queryBudget: ratelimiter.NewMemoryLimiter(ratelimiter.Config{Limit: 1000, Window: time.Minute, Prefix: "test:rq:"}),
	}
}

func assertServiceErr(t *testing.T, err error, want error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %v, got nil", want)
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != want {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

const testGroupJID = "120363000000000000@g.us"

// With group management disabled every mutation returns 404 (surface hidden).
func TestRequireGroupManagement_Disabled(t *testing.T) {
	uc := newGroupUsecase(&config.Config{GroupManagementEnabled: false})
	ctx := context.Background()

	_, err := uc.CreateGroup(ctx, "t", "628111", waDomain.CreateGroupRequest{Name: "x"})
	assertServiceErr(t, err, errDomain.ErrNotFound)
	_, err = uc.UpdateGroupParticipants(ctx, "t", "628111", waDomain.GroupParticipantsRequest{Chat: testGroupJID, Action: "remove", Participants: []string{"628222"}})
	assertServiceErr(t, err, errDomain.ErrNotFound)
	_, err = uc.JoinGroup(ctx, "t", "628111", waDomain.JoinGroupRequest{Code: "abc"})
	assertServiceErr(t, err, errDomain.ErrNotFound)
	_, err = uc.GetGroupInviteLink(ctx, "t", "628111", testGroupJID, "")
	assertServiceErr(t, err, errDomain.ErrNotFound)
	_, err = uc.ListJoinRequests(ctx, "t", "628111", testGroupJID, "")
	assertServiceErr(t, err, errDomain.ErrNotFound)
	err = uc.LinkSubGroup(ctx, "t", "628111", waDomain.CommunityLinkRequest{Chat: testGroupJID, ChildJID: testGroupJID})
	assertServiceErr(t, err, errDomain.ErrNotFound)
}

// The mass-join vector is 403 while its gate is off; empty code is 400 when on.
func TestJoinGroup_Gate(t *testing.T) {
	ctx := context.Background()

	off := newGroupUsecase(&config.Config{GroupManagementEnabled: true, GroupJoinViaLinkEnabled: false})
	_, err := off.JoinGroup(ctx, "t", "628111", waDomain.JoinGroupRequest{Code: "abc"})
	assertServiceErr(t, err, errDomain.ErrForbidden)

	on := newGroupUsecase(&config.Config{GroupManagementEnabled: true, GroupJoinViaLinkEnabled: true})
	_, err = on.JoinGroup(ctx, "t", "628111", waDomain.JoinGroupRequest{Code: "  "})
	assertBadRequest(t, err)
}

// action=add and add-on-create are 403 while the add gate is off; other actions
// pass the gate (they hit the nil manager only after it).
func TestParticipants_AddGate(t *testing.T) {
	ctx := context.Background()
	uc := newGroupUsecase(&config.Config{GroupManagementEnabled: true, GroupAddParticipantsEnabled: false, GroupMaxParticipantsPerRequest: 256})

	_, err := uc.UpdateGroupParticipants(ctx, "t", "628111", waDomain.GroupParticipantsRequest{
		Chat: testGroupJID, Action: "add", Participants: []string{"628222"},
	})
	assertServiceErr(t, err, errDomain.ErrForbidden)

	_, err = uc.CreateGroup(ctx, "t", "628111", waDomain.CreateGroupRequest{
		Name: "team", Participants: []string{"628222"},
	})
	assertServiceErr(t, err, errDomain.ErrForbidden)
}

func TestResolveGroupJID(t *testing.T) {
	if got, err := resolveGroupJID("120363000000000000@g.us", ""); err != nil || got != "120363000000000000@g.us" {
		t.Fatalf("valid group JID: got %q err %v", got, err)
	}
	// Mixed-case server is normalized/accepted.
	if got, err := resolveGroupJID("120363000000000000@G.US", ""); err != nil || got != "120363000000000000@g.us" {
		t.Fatalf("mixed-case group JID: got %q err %v", got, err)
	}
	for _, bad := range []string{"6282222222222", "6282222222222@s.whatsapp.net", "1122334455@lid", ""} {
		if _, err := resolveGroupJID(bad, ""); err == nil {
			t.Fatalf("expected 400 for %q", bad)
		} else {
			assertBadRequest(t, err)
		}
	}
}

func TestResolveParticipants(t *testing.T) {
	uc := newGroupUsecase(&config.Config{GroupMaxParticipantsPerRequest: 2})

	// Dedupe: two forms of the same number collapse to one JID.
	got, err := uc.resolveParticipants([]string{"628222", "+62 822-2", "628333"}, "628111", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "628222@s.whatsapp.net" {
		t.Fatalf("dedupe wrong: %v", got)
	}

	// Empty list with allowEmpty=false is a 400.
	if _, err := uc.resolveParticipants(nil, "628111", false, false); err == nil {
		t.Fatal("expected 400 for empty participants")
	} else {
		assertBadRequest(t, err)
	}
	// Empty list with allowEmpty=true is fine (group of just the account).
	if got, err := uc.resolveParticipants(nil, "628111", true, false); err != nil || len(got) != 0 {
		t.Fatalf("allowEmpty: got %v err %v", got, err)
	}

	// A group JID as a participant is a 400.
	if _, err := uc.resolveParticipants([]string{testGroupJID}, "628111", false, false); err == nil {
		t.Fatal("expected 400 for group participant")
	} else {
		assertBadRequest(t, err)
	}

	// Self-guard rejects the caller's own number when selfGuard=true...
	if _, err := uc.resolveParticipants([]string{"628111"}, "628111", false, true); err == nil {
		t.Fatal("expected 400 self-guard")
	} else {
		assertBadRequest(t, err)
	}
	// ...but allows it for add (selfGuard=false).
	if got, err := uc.resolveParticipants([]string{"628111"}, "628111", false, false); err != nil || len(got) != 1 {
		t.Fatalf("selfGuard off should allow own number: got %v err %v", got, err)
	}

	// Over the cap is a 400.
	if _, err := uc.resolveParticipants([]string{"628222", "628333", "628444"}, "628111", false, false); err == nil {
		t.Fatal("expected 400 over cap")
	} else {
		assertBadRequest(t, err)
	}
}

func TestParticipants_InvalidAction(t *testing.T) {
	uc := newGroupUsecase(&config.Config{GroupManagementEnabled: true, GroupMaxParticipantsPerRequest: 256})
	_, err := uc.UpdateGroupParticipants(context.Background(), "t", "628111", waDomain.GroupParticipantsRequest{
		Chat: testGroupJID, Action: "banish", Participants: []string{"628222"},
	})
	assertBadRequest(t, err)
}

func TestNameTopicSettingsLimits(t *testing.T) {
	uc := newGroupUsecase(&config.Config{GroupManagementEnabled: true})
	ctx := context.Background()

	longName := ""
	for range 26 {
		longName += "a"
	}
	assertBadRequest(t, uc.SetGroupName(ctx, "t", "628111", waDomain.SetGroupNameRequest{Chat: testGroupJID, Name: longName}))
	// Empty name is also a 400.
	assertBadRequest(t, uc.SetGroupName(ctx, "t", "628111", waDomain.SetGroupNameRequest{Chat: testGroupJID, Name: "  "}))

	longTopic := make([]byte, maxGroupTopicLen+1)
	for i := range longTopic {
		longTopic[i] = 'a'
	}
	assertBadRequest(t, uc.SetGroupTopic(ctx, "t", "628111", waDomain.SetGroupTopicRequest{Chat: testGroupJID, Topic: string(longTopic)}))

	// Settings with no flags supplied is a 400.
	_, err := uc.SetGroupSettings(ctx, "t", "628111", waDomain.GroupSettingsRequest{Chat: testGroupJID})
	assertBadRequest(t, err)
}

func TestCommunityLink_RejectsNonGroup(t *testing.T) {
	uc := newGroupUsecase(&config.Config{GroupManagementEnabled: true})
	ctx := context.Background()

	// Non-@g.us parent.
	assertBadRequest(t, uc.LinkSubGroup(ctx, "t", "628111", waDomain.CommunityLinkRequest{Chat: "628222", ChildJID: testGroupJID}))
	// Non-@g.us child.
	assertBadRequest(t, uc.LinkSubGroup(ctx, "t", "628111", waDomain.CommunityLinkRequest{Chat: testGroupJID, ChildJID: "628222"}))
	// Unlink with a bad child.
	assertBadRequest(t, uc.UnlinkSubGroup(ctx, "t", "628111", testGroupJID, "", "628222"))
}
