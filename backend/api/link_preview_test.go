package api

import (
	"bytes"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"weddinghub/models"
	"weddinghub/preview"
	"weddinghub/repository"
)

func TestShortLinkReturnsCrawlerPreviewAndLeavesInvitationPending(t *testing.T) {
	repo := repository.NewMemoryRepository()
	date := time.Date(2027, time.June, 12, 0, 0, 0, 0, time.UTC)
	wedding, err := repo.CreateWedding(models.Wedding{ID: "wedding-preview", Slug: "preview", PartnerOne: "Avery", PartnerTwo: "Jordan", Date: &date, City: "Lagos", Status: models.StatusPublished})
	if err != nil {
		t.Fatal(err)
	}
	code, codeHash, err := models.NewShortCode()
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.AddInvitation(wedding.ID, models.Invitation{ID: "inv-preview", GuestName: "Private Guest", Status: models.InvitationPending, ShortCodeHash: codeHash, CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	banner, err := preview.Render(wedding)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveWeddingBanner(wedding.ID, bannerVersion(wedding), banner); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/i/"+code, nil)
	request.Header.Set("User-Agent", "WhatsApp/2.23")
	request.Header.Set("X-Forwarded-Proto", "https")
	NewWithAllowedOrigin(repo, "").ServeHTTP(response, request)
	body := response.Body.String()
	for _, tag := range []string{`<title>Avery &amp; Jordan are getting married</title>`, `property="og:type" content="website"`, `property="og:title" content="Avery &amp; Jordan are getting married"`, `property="og:description" content="You&#39;re invited · June 12, 2027 · Lagos. Tap to view and RSVP."`, `property="og:image:width" content="1200"`, `property="og:image:height" content="630"`, `property="og:url" content="https://example.com/i/`, `name="twitter:card" content="summary_large_image"`, `https://example.com/og/wedding-preview.png?v=`} {
		if !strings.Contains(body, tag) {
			t.Fatalf("preview missing %q: %s", tag, body)
		}
	}
	if strings.Contains(body, "Private Guest") {
		t.Fatal("guest name leaked into preview")
	}
	_, invitation, err := repo.InvitationByShortCodeHash(codeHash)
	if err != nil || invitation.Status != models.InvitationPending {
		t.Fatalf("crawler changed invitation status: %+v, %v", invitation, err)
	}
	if invitation.OpenedAt != nil {
		t.Fatal("preview bot recorded an invitation open")
	}
	browser := httptest.NewRequest("GET", "/i/"+code, nil)
	browser.Header.Set("User-Agent", "Mozilla/5.0")
	browser.Header.Set("X-Forwarded-Proto", "https")
	NewWithAllowedOrigin(repo, "").ServeHTTP(httptest.NewRecorder(), browser)
	_, invitation, err = repo.InvitationByShortCodeHash(codeHash)
	if err != nil || invitation.OpenedAt == nil {
		t.Fatalf("normal browser did not record the open: %+v, %v", invitation, err)
	}
}

func TestWeddingBannerServesCachedPNG(t *testing.T) {
	repo := repository.NewMemoryRepository()
	date := time.Date(2027, time.June, 12, 0, 0, 0, 0, time.UTC)
	wedding, err := repo.CreateWedding(models.Wedding{ID: "wedding-image", Slug: "image", PartnerOne: "Avery", PartnerTwo: "Jordan", Date: &date, Status: models.StatusPublished})
	if err != nil {
		t.Fatal(err)
	}
	image, err := preview.Render(wedding)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveWeddingBanner(wedding.ID, bannerVersion(wedding), image); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	NewWithAllowedOrigin(repo, "").ServeHTTP(response, httptest.NewRequest("GET", "/og/wedding-image.png", nil))
	if response.Code != 200 || response.Header().Get("Content-Type") != "image/png" || !strings.Contains(response.Header().Get("Cache-Control"), "max-age") {
		t.Fatalf("banner response: %d, %v", response.Code, response.Header())
	}
	decoded, err := png.Decode(bytes.NewReader(response.Body.Bytes()))
	if err != nil || decoded.Bounds().Dx() != 1200 || decoded.Bounds().Dy() != 630 {
		t.Fatalf("invalid banner PNG: %v, %v", decoded, err)
	}
}

func TestUnknownShortLinkUsesGenericResponse(t *testing.T) {
	response := httptest.NewRecorder()
	NewWithAllowedOrigin(repository.NewMemoryRepository(), "").ServeHTTP(response, httptest.NewRequest("GET", "/i/AAAAAAAAAAAA", nil))
	if response.Code != 404 || !strings.Contains(response.Body.String(), "This invitation link is invalid or has expired") || strings.Contains(response.Body.String(), "og:image") {
		t.Fatalf("unexpected invalid-link response %d: %s", response.Code, response.Body.String())
	}
	unknownBody := response.Body.String()
	repo := repository.NewMemoryRepository()
	wedding, err := repo.CreateWedding(models.Wedding{ID: "wedding-revoked", Slug: "revoked", PartnerOne: "Private", PartnerTwo: "Names", Status: models.StatusPublished})
	if err != nil {
		t.Fatal(err)
	}
	code, hash, err := models.NewShortCode()
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.AddInvitation(wedding.ID, models.Invitation{ID: "revoked", GuestName: "Private Guest", Status: models.InvitationStatus("revoked"), ShortCodeHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	revoked := httptest.NewRecorder()
	NewWithAllowedOrigin(repo, "").ServeHTTP(revoked, httptest.NewRequest("GET", "/i/"+code, nil))
	if revoked.Code != 404 || revoked.Body.String() != unknownBody {
		t.Fatalf("revoked link response differs from unknown response: %d %q", revoked.Code, revoked.Body.String())
	}
}
