package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"weddinghub/models"
	"weddinghub/repository"
)

func TestOwnerWorkspaceUsesWeddingLinkedToSession(t *testing.T) {
	handler := New(repository.NewMemoryRepository())
	owner := signupSession(t, handler, "workspace@example.com")
	created := withToken(handler, http.MethodPost, "/api/weddings", owner,
		`{"slug":"workspace-couple","title":"A & B","partner_one":"A","partner_two":"B","status":"published"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create wedding status = %d: %s", created.Code, created.Body)
	}
	read := withToken(handler, http.MethodGet, "/api/owner/workspace", owner, "")
	if read.Code != http.StatusOK {
		t.Fatalf("owner workspace status = %d: %s", read.Code, read.Body)
	}
	var wedding models.Wedding
	if err := json.Unmarshal(read.Body.Bytes(), &wedding); err != nil {
		t.Fatal(err)
	}
	if wedding.Slug != "workspace-couple" || wedding.Events == nil || wedding.GuestMessages == nil {
		t.Fatalf("workspace did not return the linked wedding and empty arrays: %#v", wedding)
	}
	update := withToken(handler, http.MethodPut, "/api/owner/workspace", owner,
		`{"id":"attacker-chosen-id","slug":"attacker-slug","partner_one":"Updated","partner_two":"Couple","status":"published","venue":"Garden"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("owner workspace update status = %d: %s", update.Code, update.Body)
	}
	if got := withToken(handler, http.MethodGet, "/api/owner/workspace", owner, ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"venue":"Garden"`) {
		t.Fatalf("owner workspace update did not persist: %d %s", got.Code, got.Body)
	}
	stranger := signupSession(t, handler, "workspace-stranger@example.com")
	if got := withToken(handler, http.MethodGet, "/api/owner/workspace", stranger, ""); got.Code != http.StatusNotFound {
		t.Fatalf("unlinked user workspace status = %d, want 404", got.Code)
	}
	if got := withToken(handler, http.MethodGet, "/api/owner/workspace", "", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous workspace status = %d, want 401", got.Code)
	}
}

func TestOwnerInvitationTaskAndMessageRoutesStaySessionScoped(t *testing.T) {
	handler := New(repository.NewMemoryRepository())
	owner := signupSession(t, handler, "owner-features@example.com")
	created := withToken(handler, http.MethodPost, "/api/weddings", owner,
		`{"slug":"owner-features","title":"A & B","partner_one":"A","partner_two":"B","status":"published"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create wedding status = %d: %s", created.Code, created.Body)
	}
	inviteResponse := withToken(handler, http.MethodPost, "/api/owner/invitations", owner,
		`{"type":"guest","guest_name":"Taylor Guest","guest_email":"taylor@example.com","max_party_size":2}`)
	if inviteResponse.Code != http.StatusCreated {
		t.Fatalf("create owner invitation status = %d: %s", inviteResponse.Code, inviteResponse.Body)
	}
	var createdInvite invitationCreated
	if err := json.Unmarshal(inviteResponse.Body.Bytes(), &createdInvite); err != nil {
		t.Fatal(err)
	}
	if createdInvite.Token == "" || createdInvite.Invitation.ID == "" {
		t.Fatalf("owner invite omitted token or invitation id: %#v", createdInvite)
	}
	updatedInvite := withToken(handler, http.MethodPut, "/api/owner/invitations/"+createdInvite.Invitation.ID, owner,
		`{"guest_name":"Taylor Updated","guest_email":"taylor@example.com","max_party_size":3}`)
	if updatedInvite.Code != http.StatusOK {
		t.Fatalf("update owner invitation status = %d: %s", updatedInvite.Code, updatedInvite.Body)
	}
	rotated := withToken(handler, http.MethodPost, "/api/owner/invitations/"+createdInvite.Invitation.ID+"/refresh-link", owner, "")
	if rotated.Code != http.StatusOK {
		t.Fatalf("refresh owner link status = %d: %s", rotated.Code, rotated.Body)
	}
	var refreshed invitationCreated
	if err := json.Unmarshal(rotated.Body.Bytes(), &refreshed); err != nil || refreshed.Token == "" || refreshed.Token == createdInvite.Token {
		t.Fatalf("owner refresh did not issue a new invitation token: %#v, err=%v", refreshed, err)
	}
	accepted := withToken(handler, http.MethodPost, "/api/invitations/"+refreshed.Token+"/accept", "", "")
	if accepted.Code != http.StatusOK {
		t.Fatalf("accept owner guest invitation status = %d: %s", accepted.Code, accepted.Body)
	}
	message := withToken(handler, http.MethodPost, "/api/guest/"+refreshed.Token+"/messages", "", `{"message":"Best wishes"}`)
	if message.Code != http.StatusCreated {
		t.Fatalf("create guest message status = %d: %s", message.Code, message.Body)
	}
	var guestMessage models.GuestMessage
	if err := json.Unmarshal(message.Body.Bytes(), &guestMessage); err != nil {
		t.Fatal(err)
	}
	marked := withToken(handler, http.MethodPatch, "/api/owner/messages/"+guestMessage.ID, owner, `{"read":true}`)
	if marked.Code != http.StatusNoContent {
		t.Fatalf("mark owner message read status = %d: %s", marked.Code, marked.Body)
	}
	dashboard := withToken(handler, http.MethodGet, "/api/dashboard", owner, "")
	if dashboard.Code != http.StatusOK || !strings.Contains(dashboard.Body.String(), `"unreadMessages":0`) {
		t.Fatalf("read owner message was still counted unread: %d %s", dashboard.Code, dashboard.Body)
	}

	task := withToken(handler, http.MethodPost, "/api/owner/tasks", owner,
		`{"title":"Book florist","details":"Ask for white roses","due_on":"2026-10-12","status":"todo"}`)
	if task.Code != http.StatusCreated {
		t.Fatalf("create owner task status = %d: %s", task.Code, task.Body)
	}
	var createdTask models.PlanningTask
	if err := json.Unmarshal(task.Body.Bytes(), &createdTask); err != nil {
		t.Fatal(err)
	}
	updatedTask := withToken(handler, http.MethodPut, "/api/owner/tasks/"+createdTask.ID, owner,
		`{"title":"Book florist","details":"Done","due_on":"2026-10-12","status":"done"}`)
	if updatedTask.Code != http.StatusOK {
		t.Fatalf("update owner task status = %d: %s", updatedTask.Code, updatedTask.Body)
	}
	stranger := signupSession(t, handler, "owner-features-stranger@example.com")
	if got := withToken(handler, http.MethodDelete, "/api/owner/tasks/"+createdTask.ID, stranger, ""); got.Code != http.StatusNotFound {
		t.Fatalf("foreign owner task delete status = %d, want 404", got.Code)
	}
	if got := withToken(handler, http.MethodDelete, "/api/owner/messages/"+guestMessage.ID, owner, ""); got.Code != http.StatusNoContent {
		t.Fatalf("delete owner message status = %d: %s", got.Code, got.Body)
	}
	if got := withToken(handler, http.MethodDelete, "/api/owner/invitations/"+createdInvite.Invitation.ID, owner, ""); got.Code != http.StatusNoContent {
		t.Fatalf("delete owner invitation status = %d: %s", got.Code, got.Body)
	}
}
