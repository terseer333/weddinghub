package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"weddinghub/models"
)

type dashboardResponse struct {
	User struct {
		Name string `json:"name"`
		Role string `json:"role"`
	} `json:"user"`
	Couple struct {
		Names string `json:"names"`
		Date  string `json:"date"`
	} `json:"couple"`
	UnreadMessages int `json:"unreadMessages"`
	Guests         struct {
		Total           int `json:"total"`
		Households      int `json:"households"`
		Attending       int `json:"attending"`
		Pending         int `json:"pending"`
		Declined        int `json:"declined"`
		RecentResponses int `json:"recentResponses"`
	} `json:"guests"`
	Views struct {
		Total    int `json:"total"`
		ThisWeek int `json:"thisWeek"`
	} `json:"views"`
	Setup struct {
		Percent   int      `json:"percent"`
		Done      []string `json:"done"`
		Remaining int      `json:"remaining"`
	} `json:"setup"`
	Committee struct {
		Count         int               `json:"count"`
		TasksDone     int               `json:"tasksDone"`
		TasksTotal    int               `json:"tasksTotal"`
		TasksDueToday int               `json:"tasksDueToday"`
		Members       []dashboardMember `json:"members"`
		NextTask      *dashboardTask    `json:"nextTask"`
	} `json:"committee"`
	Activity      []dashboardActivity     `json:"activity"`
	Events        []dashboardEvent        `json:"events"`
	Announcements []dashboardAnnouncement `json:"announcements"`
	ShareURL      string                  `json:"shareUrl"`
	StoryImage    string                  `json:"storyImage"`
}

type dashboardMember struct {
	Initials string `json:"initials"`
	Color    string `json:"color"`
}
type dashboardTask struct {
	Title string `json:"title"`
	Meta  string `json:"meta"`
}
type dashboardActivity struct {
	Initials string `json:"initials"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	Time     string `json:"time"`
	Color    string `json:"color"`
}
type dashboardEvent struct {
	Day   string `json:"day"`
	Month string `json:"month"`
	Title string `json:"title"`
	Time  string `json:"time"`
	Place string `json:"place"`
}
type dashboardAnnouncement struct {
	Title string `json:"title"`
	Meta  string `json:"meta"`
	Time  string `json:"time"`
	Icon  string `json:"icon"`
}

func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	weddings := a.repo.WeddingsForUser(user.ID)
	if len(weddings) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no wedding is linked to this account"})
		return
	}
	wedding, err := a.repo.GetWedding(weddings[0].ID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	result := buildDashboard(user, wedding, a.now())
	if profile, err := a.repo.GetProfile(wedding.ID, models.ProfileKeyAdmin); err == nil && profile.DisplayName != "" {
		result.User.Name = profile.DisplayName
	}
	writeJSON(w, http.StatusOK, result)
}

func buildDashboard(user models.User, wedding models.Wedding, now time.Time) dashboardResponse {
	result := dashboardResponse{}
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = "Wedding owner"
	}
	result.User.Name, result.User.Role = name, string(user.Role)
	result.Couple.Names = strings.Trim(strings.TrimSpace(wedding.PartnerOne+" & "+wedding.PartnerTwo), "& ")
	if result.Couple.Names == "" {
		result.Couple.Names = "Your wedding"
	}
	if wedding.Date != nil {
		result.Couple.Date = wedding.Date.Format("2006-01-02")
	}
	result.Activity = make([]dashboardActivity, 0)
	result.Events = make([]dashboardEvent, 0)
	result.Announcements = make([]dashboardAnnouncement, 0)
	result.Committee.Members = make([]dashboardMember, 0)
	result.Setup.Done = make([]string, 0)
	result.ShareURL = ""
	result.StoryImage = "/assets/wedding-bg.jpg"

	guestInvitations := make(map[string]models.Invitation)
	rsvpByInvitation := make(map[string]models.RSVP)
	for _, response := range wedding.RSVPs {
		rsvpByInvitation[response.InvitationID] = response
	}
	for _, invitation := range wedding.Invitations {
		if invitation.Type.Normalized() != models.InvitationGuest {
			continue
		}
		guestInvitations[invitation.ID] = invitation
		result.Guests.Households++
		party := invitation.MaxPartySize
		if party < 1 {
			party = 1
		}
		result.Guests.Total += party
		switch invitation.Status {
		case models.InvitationPending:
			result.Guests.Pending += party
		case models.InvitationAccepted:
			response, hasResponse := rsvpByInvitation[invitation.ID]
			if !hasResponse || response.Status == models.RSVPMaybe {
				result.Guests.Pending += party
			}
		case models.InvitationDeclined:
			result.Guests.Declined += party
		}
	}
	type timedActivity struct {
		item dashboardActivity
		at   time.Time
	}
	activities := make([]timedActivity, 0)
	for _, response := range wedding.RSVPs {
		invitation, exists := guestInvitations[response.InvitationID]
		if !exists {
			continue
		}
		if response.Status == models.RSVPAttending {
			result.Guests.Attending += response.PartySize
		} else if response.Status == models.RSVPNotAttending {
			result.Guests.Declined += response.PartySize
		}
		if response.UpdatedAt.After(now.Add(-7 * 24 * time.Hour)) {
			result.Guests.RecentResponses++
		}
		activities = append(activities, timedActivity{dashboardActivity{
			Initials: nameInitials(invitation.GuestName), Name: invitation.GuestName,
			Text: "responded to the invitation", Time: relativeDashboardTime(response.UpdatedAt, now), Color: colorForName(invitation.GuestName),
		}, response.UpdatedAt})
	}
	for _, message := range wedding.GuestMessages {
		if !message.Read {
			result.UnreadMessages++
		}
		invitation := guestInvitations[message.InvitationID]
		activities = append(activities, timedActivity{dashboardActivity{
			Initials: nameInitials(invitation.GuestName), Name: invitation.GuestName,
			Text: "left a guest message", Time: relativeDashboardTime(message.CreatedAt, now), Color: colorForName(invitation.GuestName),
		}, message.CreatedAt})
	}
	sort.Slice(activities, func(i, j int) bool { return activities[i].at.After(activities[j].at) })
	for i, activity := range activities {
		if i == 5 {
			break
		}
		result.Activity = append(result.Activity, activity.item)
	}

	for _, member := range wedding.CommitteeMembers {
		result.Committee.Members = append(result.Committee.Members, dashboardMember{Initials: nameInitials(member.Name), Color: colorForName(member.Name)})
	}
	result.Committee.Count = len(wedding.CommitteeMembers)
	today := now.Format("2006-01-02")
	var openTasks []models.PlanningTask
	for _, task := range wedding.PlanningTasks {
		result.Committee.TasksTotal++
		if task.Status == models.TaskDone {
			result.Committee.TasksDone++
		} else {
			openTasks = append(openTasks, task)
			if task.DueOn == today {
				result.Committee.TasksDueToday++
			}
		}
	}
	if len(openTasks) > 0 {
		sort.Slice(openTasks, func(i, j int) bool {
			if openTasks[i].DueOn == openTasks[j].DueOn {
				return openTasks[i].CreatedAt.Before(openTasks[j].CreatedAt)
			}
			if openTasks[i].DueOn == "" {
				return false
			}
			if openTasks[j].DueOn == "" {
				return true
			}
			return openTasks[i].DueOn < openTasks[j].DueOn
		})
		result.Committee.NextTask = &dashboardTask{Title: openTasks[0].Title, Meta: openTasks[0].DueOn}
	}

	events := append([]models.Event(nil), wedding.Events...)
	sort.Slice(events, func(i, j int) bool { return events[i].StartsAt.Before(events[j].StartsAt) })
	for _, event := range events {
		if event.Status != models.StatusPublished || event.StartsAt.Before(now) {
			continue
		}
		result.Events = append(result.Events, dashboardEvent{Day: event.StartsAt.Format("02"), Month: strings.ToUpper(event.StartsAt.Format("Jan")), Title: event.Name, Time: event.StartsAt.Format("3:04 PM"), Place: event.Venue})
		if len(result.Events) == 4 {
			break
		}
	}
	announcements := append([]models.Announcement(nil), wedding.Announcements...)
	sort.Slice(announcements, func(i, j int) bool { return announcements[i].CreatedAt.After(announcements[j].CreatedAt) })
	for _, announcement := range announcements {
		if announcement.Status != models.StatusPublished {
			continue
		}
		at := announcement.CreatedAt
		if announcement.PublishedAt != nil {
			at = *announcement.PublishedAt
		}
		result.Announcements = append(result.Announcements, dashboardAnnouncement{Title: announcement.Title, Meta: announcement.Body, Time: relativeDashboardTime(at, now), Icon: "megaphone"})
	}
	if len(result.Announcements) > 3 {
		result.Announcements = result.Announcements[:3]
	}
	photoImageFound := false
	for _, photo := range wedding.Photos {
		if photo.URL != "" {
			result.StoryImage = photo.URL
			photoImageFound = true
			break
		}
	}
	if !photoImageFound {
		for _, story := range wedding.StorySections {
			if story.PhotoURL != "" {
				result.StoryImage = story.PhotoURL
				break
			}
		}
	}

	setupItems := []struct {
		key  string
		done bool
	}{
		{"Couple names", wedding.PartnerOne != "" && wedding.PartnerTwo != ""},
		{"Wedding date", wedding.Date != nil},
		{"Venue", wedding.Venue != ""},
		{"Invitation design", wedding.TemplateID != ""},
		{"Wedding story", len(wedding.StorySections) > 0},
	}
	for _, item := range setupItems {
		if item.done {
			result.Setup.Done = append(result.Setup.Done, item.key)
		}
	}
	result.Setup.Remaining = len(setupItems) - len(result.Setup.Done)
	result.Setup.Percent = len(result.Setup.Done) * 100 / len(setupItems)
	return result
}

func nameInitials(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return "?"
	}
	first := []rune(fields[0])
	initials := strings.ToUpper(string(first[0]))
	if len(fields) > 1 {
		second := []rune(fields[len(fields)-1])
		initials += strings.ToUpper(string(second[0]))
	}
	return initials
}

func colorForName(name string) string {
	colors := []string{"rose", "sage", "gold", "blue"}
	if name == "" {
		return colors[0]
	}
	return colors[int([]rune(name)[0])%len(colors)]
}

func relativeDashboardTime(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	delta := now.Sub(at)
	if delta < time.Minute {
		return "just now"
	}
	if delta < time.Hour {
		return pluralDashboard(int(delta.Minutes()), "min")
	}
	if delta < 24*time.Hour {
		return pluralDashboard(int(delta.Hours()), "hr")
	}
	if delta < 48*time.Hour {
		return "Yesterday"
	}
	return pluralDashboard(int(delta.Hours()/24), "day")
}

func pluralDashboard(n int, unit string) string {
	return strings.TrimSpace(strings.Join([]string{itoaDashboard(n), unit + func() string {
		if n == 1 {
			return ""
		}
		return "s"
	}()}, " "))
}

func itoaDashboard(n int) string { return strconv.Itoa(n) }
