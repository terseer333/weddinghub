package preview

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"weddinghub/models"
)

func TestRenderProducesBounded1200By630PNG(t *testing.T) {
	date := time.Date(2027, time.June, 12, 0, 0, 0, 0, time.UTC)
	data, err := Render(models.Wedding{PartnerOne: "Avery", PartnerTwo: "Jordan", Date: &date, City: "Lagos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(data) >= 300*1024 {
		t.Fatalf("image size = %d", len(data))
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != Width || decoded.Bounds().Dy() != Height {
		t.Fatalf("image size = %v", decoded.Bounds())
	}
}
