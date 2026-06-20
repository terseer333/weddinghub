package models

type Event struct {
	ID            int      `json:"id"`
	BrideName     string   `json:"bride_name"`
	GroomName     string   `json:"groom_name"`
	EventDate     string   `json:"event_date"`
	Venue         string   `json:"venue"`
	Description   string   `json:"description"`
	HeroImage     string   `json:"hero_image"`
	GalleryImages []string `json:"gallery_images"`
}
